# Self-Hosted MongoDB

Drop-in replacement for MongoDB Atlas. Change streams work (replica set auto-initialises), so SSE and live config reload require no code changes.

---

## Step 1 — Get a VM

**Recommended free option: Oracle Cloud Always Free**

1. Sign up at [cloud.oracle.com](https://cloud.oracle.com)
2. Create an **Ampere A1 ARM** instance — 2 OCPU + 8 GB RAM, Ubuntu 22.04, always free
3. Download the SSH key when prompted
4. In **Networking → Security List**, open port **22** only — keep 27017 closed
5. SSH in: `ssh -i ~/your-key.pem ubuntu@<VM_IP>`

---

## Step 2 — Install Docker on the VM

```bash
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
# Log out and back in, then verify:
docker compose version
```

---

## Step 3 — Copy and configure

From your local machine:

```bash
scp -i ~/your-key.pem -r docker/mongodb ubuntu@<VM_IP>:~/saasquickstart-mongo
```

On the VM:

```bash
cd ~/saasquickstart-mongo
cp .env.example .env
nano .env   # set MONGO_USER, MONGO_PASS, MONGO_DB
```

Generate the replica set keyfile (once only — back this file up):

```bash
openssl rand -base64 756 > keyfile && chmod 400 keyfile
```

---

## Step 4 — Start

```bash
docker compose up -d
```

This starts MongoDB, waits for it to be healthy, then `mongo-init` auto-initialises the replica set and exits. The `mongo-backup` container runs daily at 02:00.

Verify:

```bash
docker compose ps
docker logs saasquickstart-mongo-init
```

---

## Step 5 — Point your app at the new database

Add to Fly secrets:

```bash
fly secrets set MONGODB_URI="mongodb://USER:PASS@<VM_IP>:27017/DB?replicaSet=rs0&authSource=admin" \
  -c fly.saas.toml
```

Or in `backend/config/prod.yaml`:

```yaml
mongodb_uri: "mongodb://USER:PASS@<VM_IP>:27017/DB?replicaSet=rs0&authSource=admin"
```

No code changes. Restart the app and it connects to the new host.

---

## Falling back to Atlas

Revert `MONGODB_URI` to your Atlas connection string and restart. Atlas is always your fallback.

---

## Backups

Daily at 02:00, kept 7 days locally. Set `S3_BUCKET` + credentials in `.env` to also upload to Cloudflare R2 or S3.

Run a manual backup anytime:

```bash
docker exec saasquickstart-mongo-backup /scripts/backup.sh
```

---

## Security checklist

- [ ] `keyfile` backed up (losing it means you cannot restart the replica set)
- [ ] `.env` not committed to git (already in `.gitignore`)
- [ ] Port 27017 closed to the public internet
- [ ] Use [Tailscale](https://tailscale.com) for a private tunnel between the VM and your app
- [ ] `MONGO_PASS` is a strong random password

---

## Useful commands

```bash
docker compose logs -f mongo                              # live logs
docker exec saasquickstart-mongo-backup /scripts/backup.sh     # manual backup
docker exec -it saasquickstart-mongo mongosh                   # open a shell
docker compose down                                       # stop (data preserved)
docker compose up -d                                      # restart after VM reboot
```
