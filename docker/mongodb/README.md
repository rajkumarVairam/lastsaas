# Self-Hosted MongoDB

Drop-in replacement for MongoDB Atlas. Change streams work (replica set is auto-initialised), so SSE and live config reload work without modification.

## First-time VM setup (any provider)

```bash
# 1. Clone or copy this directory onto your VM
scp -r docker/mongodb user@<VM_IP>:~/lastsaas-mongo

# 2. On the VM:
cd ~/lastsaas-mongo
cp .env.example .env
nano .env          # set MONGO_USER, MONGO_PASS, MONGO_DB

# 3. Generate the replica set keyfile (once only — back it up)
bash generate-keyfile.sh

# 4. Start everything
docker compose up -d

# Replica set is initialised automatically. Check logs:
docker logs lastsaas-mongo-init
docker logs lastsaas-mongo-backup
```

## Switching your app from Atlas → self-hosted

In your `backend/config/prod.yaml` (or Fly secrets), change one value:

```yaml
# Atlas (current)
mongodb_uri: "mongodb+srv://user:pass@cluster.mongodb.net/?retryWrites=true"

# Self-hosted (single-node replica set)
mongodb_uri: "mongodb://lastsaas:PASS@<VM_IP>:27017/lastsaas?replicaSet=rs0&authSource=admin"
```

No code changes. Restart the app and it connects to the new host.

## Falling back to Atlas

Just revert `mongodb_uri` to your Atlas connection string and restart. Atlas remains your fallback at all times.

## Backups

| What | Where | Schedule |
|---|---|---|
| Local archive | `mongo_backups` Docker volume | Daily 02:00 |
| Offsite | S3 or Cloudflare R2 | Same, if `S3_BUCKET` is set in `.env` |
| Retention | Configurable via `BACKUP_RETAIN_DAYS` | Default: 7 days |

Run a manual backup anytime:
```bash
docker exec lastsaas-mongo-backup /scripts/backup.sh
```

## Recommended free VM: Oracle Cloud Always Free

- Sign up at cloud.oracle.com
- Create an **Ampere A1 ARM** instance (4 OCPU + 24 GB RAM — always free)
- Open port 27017 only in the VCN security list **if** using Tailscale/WireGuard between app and DB
- Otherwise keep `127.0.0.1:27017` binding (default) and tunnel via SSH or Tailscale

## Security checklist

- [ ] `keyfile` backed up securely (losing it = cannot restart replica set)
- [ ] `.env` not committed to git (already in `.gitignore`)
- [ ] Port 27017 is **not** open to the public internet — use Tailscale or SSH tunnel
- [ ] Offsite backup configured (`S3_BUCKET` in `.env`)
- [ ] `MONGO_PASS` is a strong random password (not the example value)
