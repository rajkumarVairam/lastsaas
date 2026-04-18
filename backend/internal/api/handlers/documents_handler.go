package handlers

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"lastsaas/internal/db"
	"lastsaas/internal/middleware"
	"lastsaas/internal/models"
	"lastsaas/internal/objectstore"
	"lastsaas/internal/syslog"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const maxDocumentSize = 50 << 20 // 50 MB

// DocumentsHandler provides tenant-scoped private file storage.
// Files are namespaced in the object store as "documents/{tenantId}/{docId}"
// so they are never reachable without an authenticated presign request.
type DocumentsHandler struct {
	db       *db.MongoDB
	objStore objectstore.Store
	syslog   *syslog.Logger
}

func NewDocumentsHandler(database *db.MongoDB, store objectstore.Store, sysLogger *syslog.Logger) *DocumentsHandler {
	return &DocumentsHandler{db: database, objStore: store, syslog: sysLogger}
}

// ListDocuments returns documents visible to the caller within the tenant.
// Query params: ?mine=true to restrict to own uploads; ?page=1
func (h *DocumentsHandler) ListDocuments(w http.ResponseWriter, r *http.Request) {
	tenant, _ := middleware.GetTenantFromContext(r.Context())
	user, _ := middleware.GetUserFromContext(r.Context())

	filter := bson.M{"tenantId": tenant.ID}

	if r.URL.Query().Get("mine") == "true" {
		filter["ownerId"] = user.ID
	} else {
		// Tenant-visible docs + caller's own private docs
		filter["$or"] = bson.A{
			bson.M{"visibility": models.DocumentVisibilityTenant},
			bson.M{"ownerId": user.ID},
		}
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit := int64(50)

	opts := options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetLimit(limit).
		SetSkip(int64(page-1) * limit).
		SetProjection(bson.M{"data": 0}) // never return raw bytes

	cursor, err := h.db.Documents().Find(r.Context(), filter, opts)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to list documents")
		return
	}
	var docs []models.Document
	if err := cursor.All(r.Context(), &docs); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to decode documents")
		return
	}
	if docs == nil {
		docs = []models.Document{}
	}
	respondWithJSON(w, http.StatusOK, map[string]interface{}{"documents": docs})
}

// UploadDocument handles a multipart file upload scoped to the tenant.
func (h *DocumentsHandler) UploadDocument(w http.ResponseWriter, r *http.Request) {
	tenant, _ := middleware.GetTenantFromContext(r.Context())
	user, _ := middleware.GetUserFromContext(r.Context())

	if err := r.ParseMultipartForm(maxDocumentSize); err != nil {
		respondWithError(w, http.StatusBadRequest, "File too large (max 50MB)")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Missing file upload")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxDocumentSize+1))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to read file")
		return
	}
	if int64(len(data)) > maxDocumentSize {
		respondWithError(w, http.StatusBadRequest, "File too large (max 50MB)")
		return
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	visibility := models.DocumentVisibility(r.FormValue("visibility"))
	if visibility != models.DocumentVisibilityOwner {
		visibility = models.DocumentVisibilityTenant // safe default
	}

	now := time.Now()
	id := primitive.NewObjectID()
	// Namespace by tenant so all tenant files are partitioned cleanly.
	storageKey := fmt.Sprintf("documents/%s/%s", tenant.ID.Hex(), id.Hex())

	doc := models.Document{
		ID:          id,
		TenantID:    tenant.ID,
		OwnerID:     user.ID,
		Filename:    header.Filename,
		ContentType: contentType,
		Size:        int64(len(data)),
		Visibility:  visibility,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if h.objStore != nil && h.objStore.Provider() != "db" {
		if _, storeErr := h.objStore.Put(r.Context(), storageKey, data, contentType); storeErr != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to upload document to storage")
			return
		}
		// Store only the key — the URL is re-derived via PresignGet at download time.
		doc.StorageKey = storageKey
	} else {
		doc.Data = data
	}

	if _, err := h.db.Documents().InsertOne(r.Context(), doc); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to save document")
		return
	}

	h.syslog.Low(r.Context(), fmt.Sprintf("Document uploaded: %s (%s, %d bytes) tenant=%s", header.Filename, contentType, len(data), tenant.ID.Hex()))
	doc.Data = nil // strip bytes from response
	respondWithJSON(w, http.StatusCreated, doc)
}

// GetDocument returns document metadata (no file bytes).
func (h *DocumentsHandler) GetDocument(w http.ResponseWriter, r *http.Request) {
	doc, ok := h.fetchAuthorized(w, r)
	if !ok {
		return
	}
	doc.Data = nil
	respondWithJSON(w, http.StatusOK, doc)
}

// DownloadDocument issues a short-lived presigned redirect.
// On the db-fallback provider (local dev) it streams bytes directly.
func (h *DocumentsHandler) DownloadDocument(w http.ResponseWriter, r *http.Request) {
	doc, ok := h.fetchAuthorized(w, r)
	if !ok {
		return
	}

	if h.objStore != nil && doc.StorageKey != "" {
		signed, err := h.objStore.PresignGet(r.Context(), doc.StorageKey, 15*time.Minute, doc.Filename)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to generate download link")
			return
		}
		if signed != "" {
			// 302 — must not be cached; URL expires in 15 min.
			http.Redirect(w, r, signed, http.StatusFound)
			return
		}
	}

	// db-fallback: stream from MongoDB directly.
	if len(doc.Data) == 0 {
		respondWithError(w, http.StatusNotFound, "File data not found")
		return
	}
	safe := strings.ReplaceAll(doc.Filename, `"`, `'`)
	w.Header().Set("Content-Type", doc.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, safe))
	w.Header().Set("Content-Length", strconv.FormatInt(doc.Size, 10))
	w.Write(doc.Data)
}

// DeleteDocument removes the document from storage and MongoDB.
// Only the uploader or a tenant admin/owner can delete.
func (h *DocumentsHandler) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	tenant, _ := middleware.GetTenantFromContext(r.Context())
	user, _ := middleware.GetUserFromContext(r.Context())

	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid document ID")
		return
	}

	var doc models.Document
	if err := h.db.Documents().FindOne(r.Context(), bson.M{"_id": id, "tenantId": tenant.ID}).Decode(&doc); err == mongo.ErrNoDocuments {
		respondWithError(w, http.StatusNotFound, "Document not found")
		return
	} else if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch document")
		return
	}

	// Only the uploader or a tenant admin/owner may delete.
	var membership models.TenantMembership
	memErr := h.db.TenantMemberships().FindOne(r.Context(), bson.M{"userId": user.ID, "tenantId": tenant.ID}).Decode(&membership)
	isAdminOrAbove := memErr == nil && (membership.Role == models.RoleAdmin || membership.Role == models.RoleOwner)
	if doc.OwnerID != user.ID && !isAdminOrAbove {
		respondWithError(w, http.StatusForbidden, "Only the uploader or a tenant admin can delete this document")
		return
	}

	if h.objStore != nil && doc.StorageKey != "" {
		_ = h.objStore.Delete(r.Context(), doc.StorageKey)
	}

	if _, err := h.db.Documents().DeleteOne(r.Context(), bson.M{"_id": id, "tenantId": tenant.ID}); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to delete document")
		return
	}

	h.syslog.Low(r.Context(), fmt.Sprintf("Document deleted: %s tenant=%s", doc.Filename, tenant.ID.Hex()))
	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// fetchAuthorized loads the document and enforces tenant isolation + visibility.
func (h *DocumentsHandler) fetchAuthorized(w http.ResponseWriter, r *http.Request) (*models.Document, bool) {
	tenant, _ := middleware.GetTenantFromContext(r.Context())
	user, _ := middleware.GetUserFromContext(r.Context())

	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid document ID")
		return nil, false
	}

	var doc models.Document
	err = h.db.Documents().FindOne(r.Context(), bson.M{"_id": id, "tenantId": tenant.ID}).Decode(&doc)
	if err == mongo.ErrNoDocuments {
		respondWithError(w, http.StatusNotFound, "Document not found")
		return nil, false
	} else if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch document")
		return nil, false
	}

	// Owner-only documents are not visible to other tenant members.
	if doc.Visibility == models.DocumentVisibilityOwner && doc.OwnerID != user.ID {
		respondWithError(w, http.StatusForbidden, "Access denied")
		return nil, false
	}

	return &doc, true
}
