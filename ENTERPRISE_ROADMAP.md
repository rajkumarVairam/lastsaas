# Enterprise Features Roadmap

These are the core enhancements necessary to elevate the product from SMB/Mid-Market SaaS to a true Enterprise-grade application.

---

## Priority 1 (High): Enterprise Identity & SSO

**Feature:** Implement SSO (SAML 2.0 / OIDC) and SCIM Directory Sync

**Description:**
Integrate enterprise identity providers (Okta, Azure AD, Ping Identity) to support true Enterprise Single Sign-On (SAML 2.0 / OIDC). Additionally, implement SCIM protocol for automated directory sync so enterprise tenants can provision and de-provision user accounts automatically based on their Active Directory.

**Acceptance Criteria:**
- [ ] Support SAML 2.0 and OIDC connections per-tenant.
- [ ] Implement SCIM endpoints for user provisioning/deprovisioning.
- [ ] Admin UI for tenants to configure their identity provider settings.

**Suggested Approach:**
Evaluate using WorkOS to abstract the complexity of SAML/SCIM integrations instead of building from scratch.

---

## Priority 2 (High): Advanced Security & Access Controls

**Feature:** Advanced Role-Based Security & IP Restrictions

**Description:**
Current access controls (Owner/Admin/User) need to be expanded for enterprise compliance.
- **Custom Roles**: Permit enterprise tenants to define roles like "Billing Manager" or "Read-only Auditor" with granular permission matrices.
- **IP Allowlisting**: Allow tenants to restrict logins and API key requests strictly to their corporate office/VPN CIDR blocks.
- **Enforced MFA**: Ensure tenant owners can toggle a setting that requires all child users to have MFA enabled.

**Acceptance Criteria:**
- [ ] Custom Role builder UI and database schema implementation.
- [ ] IP restriction middleware scoped to tenants.
- [ ] "Enforce MFA" toggle mechanism for tenant level.

---

## Priority 3 (Medium): Compliance & Data Governance

**Feature:** Compliance Tooling (Audit Exports & Data Residency)

**Description:**
Enterprises require advanced auditability and data governance features to pass compliance.
- **SIEM Export**: Implement a mechanism to forward audit logs to enterprise tools like Splunk or Datadog automatically.
- **Bring Your Own Key (BYOK)**: Allow enterprise IT to manage their own database encryption keys so the vendor cannot access raw data.
- **Data Residency**: Abstraction to guarantee specific tenant data remains in specific AWS regions (e.g., EU vs US).

**Acceptance Criteria:**
- [ ] Configurable audit log streaming module (e.g., via EventBridge).
- [ ] Proof of concept for application-layer encryption utilizing tenant keys (BYOK).

---

## Priority 4 (Medium): Enterprise Billing Variations

**Feature:** Custom Invoicing, Net-30 Terms & SLAs

**Description:**
Enterprise clients often bypass credit-card-based subscription platforms heavily utilized in the current setup. We need to implement custom billing modules outside of the standard Stripe checkout loop.
- **Custom Invoicing & POs**: Add parsing for Purchase Order (PO) numbers and net-30/60 custom invoicing workflows.
- **Custom SLAs**: Integrate Service Level Agreement (SLA) status tags per tenant directly connected to reporting.

**Acceptance Criteria:**
- [ ] Option to bypass Stripe automatic collection for specific high-value tenants.
- [ ] UI for generating, managing, and resolving manual Enterprise invoices.
