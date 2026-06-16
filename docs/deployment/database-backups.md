# Database Backups

PostgreSQL is the durable source of truth for users, rooms, memberships, and game history. Back it up daily and copy each backup off the VPS.

Redis is not the primary backup target. The Redis services hold OAuth state, presence, room snapshots, owner leases, and pub/sub relay state. They use AOF persistence for restart continuity, but PostgreSQL is the data store that needs long-term backup and restore procedures.

## Backup Goals

- Create a compressed PostgreSQL dump every day.
- Store one local copy on the VPS for quick restores.
- Upload every backup to S3-compatible object storage.
- Retain local backups for 30 days.
- Use bucket lifecycle rules or `rclone delete` for remote retention.
- Run periodic restore drills.
- Keep backup files and credentials out of the repo.

## Install Tools

Install `rclone` on the VPS:

```bash
sudo apt update
sudo apt install -y rclone
```

Configure an S3-compatible remote for root, because the backup script is installed under `/opt/7spade` and scheduled from root's crontab:

```bash
sudo rclone config
```

Use a remote name such as `s3`. `rclone` supports AWS S3, Cloudflare R2, Backblaze B2, DigitalOcean Spaces, MinIO, and other S3-compatible providers.

Verify the remote:

```bash
sudo rclone lsd s3:
```

## Backup Environment

Create `/opt/7spade/backup.env`:

```env
BACKUP_DIR=/opt/backups/7spade/postgres
RCLONE_REMOTE=s3:7spade-backups/postgres
LOCAL_RETENTION_DAYS=30
REMOTE_RETENTION_DAYS=90
```

Lock it down:

```bash
sudo chmod 600 /opt/7spade/backup.env
```

Use least-privilege object-storage credentials. The S3 user should only be able to write, list, read, and delete objects in the backup bucket or prefix.

## Backup Script

Create `/opt/7spade/backup-postgres.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

source /opt/7spade/backup.env

timestamp=$(date -u +%Y%m%d-%H%M%S)
backup_name="sevens-${timestamp}.sql.gz"
backup_file="${BACKUP_DIR}/${backup_name}"

mkdir -p "${BACKUP_DIR}"

cid=$(docker ps -q -f name=7spade_postgres | head -n 1)
if [ -z "${cid}" ]; then
  echo "postgres container not found" >&2
  exit 1
fi

docker exec -i "${cid}" pg_dump -U sevens sevens | gzip > "${backup_file}"

rclone copy "${backup_file}" "${RCLONE_REMOTE}/"

find "${BACKUP_DIR}" -name "sevens-*.sql.gz" -mtime +"${LOCAL_RETENTION_DAYS}" -delete

if [ -n "${REMOTE_RETENTION_DAYS:-}" ]; then
  rclone delete "${RCLONE_REMOTE}/" --min-age "${REMOTE_RETENTION_DAYS}d"
  rclone rmdirs "${RCLONE_REMOTE}/" --leave-root
fi

echo "backup completed: ${backup_name}"
```

Make it executable:

```bash
sudo chmod 700 /opt/7spade/backup-postgres.sh
```

Run a manual backup:

```bash
sudo /opt/7spade/backup-postgres.sh
```

Verify the local file exists:

```bash
ls -lh /opt/backups/7spade/postgres
```

Verify the remote upload exists:

```bash
sudo rclone ls s3:7spade-backups/postgres
```

## Schedule With Cron

Open root's crontab:

```bash
sudo crontab -e
```

Run the backup daily at 03:00 server time:

```cron
0 3 * * * /opt/7spade/backup-postgres.sh >> /var/log/7spade-postgres-backup.log 2>&1
```

If the server timezone is not UTC, document the timezone in the operations notes or change the cron time accordingly.

## Restore From Local Backup

Restoring over production replaces current database state. Take a fresh pre-restore backup first and perform the restore during a maintenance window.

Create a pre-restore backup:

```bash
sudo /opt/7spade/backup-postgres.sh
```

Restore a local dump:

```bash
cid=$(docker ps -q -f name=7spade_postgres | head -n 1)
gunzip -c /opt/backups/7spade/postgres/sevens-YYYYmmdd-HHMMSS.sql.gz | docker exec -i "$cid" psql -U sevens sevens
```

For a cleaner restore into an existing database, drop and recreate the public schema first:

```bash
cid=$(docker ps -q -f name=7spade_postgres | head -n 1)
docker exec -i "$cid" psql -U sevens sevens -c 'DROP SCHEMA public CASCADE; CREATE SCHEMA public;'
gunzip -c /opt/backups/7spade/postgres/sevens-YYYYmmdd-HHMMSS.sql.gz | docker exec -i "$cid" psql -U sevens sevens
```

After restore, restart the API so migrations and application startup checks run against the restored database:

```bash
docker service update --force --with-registry-auth 7spade_api
```

## Restore From S3-Compatible Storage

List remote backups:

```bash
sudo rclone lsf s3:7spade-backups/postgres
```

Download one backup:

```bash
sudo rclone copy s3:7spade-backups/postgres/sevens-YYYYmmdd-HHMMSS.sql.gz /tmp/
```

Restore it:

```bash
cid=$(docker ps -q -f name=7spade_postgres | head -n 1)
gunzip -c /tmp/sevens-YYYYmmdd-HHMMSS.sql.gz | docker exec -i "$cid" psql -U sevens sevens
```

## Destination Options

S3-compatible storage is the default backup destination for production.

| Destination | Use For | Pros | Cons |
|---|---|---|---|
| S3-compatible object storage | Primary production backups | Durable, cheap, supports lifecycle retention | Requires credentials and setup |
| Another VPS over SSH/rsync | Secondary copy | Simple, cloud-independent | You own disk durability and security |
| Telegram bot | Notifications only | Fast mobile visibility | Not suitable as the only production backup store |
| GitHub artifacts/releases | Avoid for DB backups | Convenient | Sensitive data risk and retention limits |
| Email attachment | Avoid except tiny dev backups | Simple | Size, security, and deliverability problems |

## Telegram Notifications Optional

Telegram is useful for success/failure notifications, but S3-compatible storage should remain the primary backup destination.

Add optional variables to `/opt/7spade/backup.env`:

```env
TELEGRAM_BOT_TOKEN=<redacted>
TELEGRAM_CHAT_ID=<redacted>
```

Add this helper to `backup-postgres.sh` if notifications are desired:

```bash
notify_telegram() {
  if [ -z "${TELEGRAM_BOT_TOKEN:-}" ] || [ -z "${TELEGRAM_CHAT_ID:-}" ]; then
    return 0
  fi

  curl -sS -X POST "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/sendMessage" \
    -d chat_id="${TELEGRAM_CHAT_ID}" \
    -d text="$1" >/dev/null
}
```

Then call it after a successful upload:

```bash
notify_telegram "7spade backup completed: ${backup_name}"
```

Telegram document uploads are intentionally not the default. Database dumps can contain sensitive user data and may outgrow Telegram file limits.

## Security Notes

- Keep `/opt/7spade/backup.env` mode `600`.
- Keep `/opt/7spade/backup-postgres.sh` mode `700`.
- Do not store backups under the repo checkout.
- Do not commit rclone config, S3 credentials, dumps, or Telegram tokens.
- Prefer bucket lifecycle rules for remote retention when the provider supports them.
- Enable provider-side encryption if available.
- Consider client-side encryption before upload if the object-storage provider is not fully trusted.

Example client-side encryption:

```bash
gpg --symmetric --cipher-algo AES256 "${backup_file}"
rclone copy "${backup_file}.gpg" "${RCLONE_REMOTE}/"
```

If client-side encryption is enabled, store the passphrase in a secure password manager. A backup encrypted with a lost passphrase is not restorable.

## Restore Drill

Run a restore drill at least monthly:

1. Download the latest S3 backup to a disposable environment.
2. Restore into a fresh PostgreSQL database or disposable stack.
3. Verify expected tables exist.
4. Run a small read-only application smoke check if practical.
5. Record the backup timestamp, restore duration, and any issues.

An untested backup should be treated as unproven.
