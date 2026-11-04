# Object Storage

## Purpose

File storage for large binary objects such as documents.

## Planned use in GovAI

- Stores the original government documents.

## Local setup (SeaweedFS)

Object storage runs locally with [SeaweedFS](https://github.com/seaweedfs/seaweedfs),
which provides an S3-compatible API. It is defined as the `seaweedfs` service in the
root `docker-compose.yml`.

| Interface | URL | Use |
| --- | --- | --- |
| S3 API | http://localhost:8333 | Used by GovAI services to store and read documents |
| Filer UI | http://localhost:8888 | Browse stored files in the browser |
| Master UI | http://localhost:9333 | Monitor storage status |
| Admin UI | http://localhost:23646 | Dashboard: cluster, buckets, file browser, volumes, maintenance (login required) |

All ports are bound to `127.0.0.1`, so they are only reachable from your machine.

### Prerequisites

- Docker Desktop installed and running
- A `.env` file in the repository root

### 1. Configure environment variables

From the repository root, create `.env` once if it does not exist:

```powershell
Copy-Item .env.example .env
```

Check the object storage settings in `.env`:

```env
SEAWEEDFS_S3_PORT=8333
OBJECT_STORAGE_ENDPOINT=http://localhost:8333
OBJECT_STORAGE_BUCKET=govai-knowledge
OBJECT_STORAGE_REGION=ap-south-1
OBJECT_STORAGE_ACCESS_KEY=govai_access_key
OBJECT_STORAGE_SECRET_KEY=change_this_object_storage_secret
```

Change `OBJECT_STORAGE_SECRET_KEY` to your own value. Startup fails if the access key
or secret key is missing.

For the Admin UI, also set its login password:

```env
SEAWEEDFS_ADMIN_PASSWORD=change_this_seaweedfs_admin_password
```

The Admin UI does not start if `SEAWEEDFS_ADMIN_PASSWORD` is missing.

### 2. Start SeaweedFS

From the repository root:

```powershell
docker compose up -d seaweedfs
```

Check that it is running:

```powershell
docker compose ps seaweedfs
docker compose logs -f seaweedfs
```

Press Ctrl+C to stop following the logs. The container keeps running.

### 3. Create the bucket (first time only)

SeaweedFS does not create buckets automatically. Create the GovAI bucket once:

```powershell
docker compose exec seaweedfs sh -c "echo 's3.bucket.create -name govai-knowledge' | weed shell"
```

Confirm that it exists:

```powershell
docker compose exec seaweedfs sh -c "echo 's3.bucket.list' | weed shell"
```

The bucket is stored in the `seaweedfs_data` volume, so this step is not needed again
unless the volume is deleted.

### 4. Verify

- Open http://localhost:8888 and go to `/buckets/govai-knowledge`.
- Open http://localhost:9333 to see the storage status.
- Run `curl.exe http://localhost:8333`. An XML `AccessDenied` response means the S3 API
  is running and requires credentials.

Links to the volume server in the Master UI use the container's internal address and do
not open from the browser. This is expected for the local single-container setup.

### Admin UI (optional)

The Admin UI is a separate container, `seaweedfs-admin`, that connects to the master and
finds the filer by itself. Start it after SeaweedFS:

```powershell
docker compose up -d seaweedfs-admin
```

Open http://localhost:23646 and log in with user `admin` and the
`SEAWEEDFS_ADMIN_PASSWORD` from the root `.env`.

| Page | URL | Use |
| --- | --- | --- |
| Dashboard | http://localhost:23646/admin | Health of the master, volume server, and filer |
| Buckets | http://localhost:23646/object-store/buckets | Buckets, their size and object count |
| File browser | http://localhost:23646/files | Browse `/buckets/govai-knowledge/documents/...` |
| Volumes | http://localhost:23646/storage/volumes | Volume usage |
| Cluster | http://localhost:23646/cluster | Master, volume servers, filers, and S3 gateway |

Use it to look at documents, not to upload them: GovAI documents must be uploaded with
the knowledge service's `upload_documents.py`, which gives each PDF its content-based
key and metadata.

Its settings are kept in the `seaweedfs_admin_data` volume. Stop it with
`docker compose stop seaweedfs-admin`.

### Stop

```powershell
docker compose stop seaweedfs
```

Stored files remain in the `seaweedfs_data` volume.

### Troubleshooting

| Problem | Fix |
| --- | --- |
| `Set OBJECT_STORAGE_ACCESS_KEY in .env` | Add the missing key to the root `.env` |
| Port already in use | Stop the other program using 8333, 8888, 9333 or 23646, or change `SEAWEEDFS_S3_PORT` for the S3 API |
| `Set SEAWEEDFS_ADMIN_PASSWORD in .env` | Add the Admin UI password to the root `.env` |
| Admin UI shows no filer | Check that `seaweedfs` is running, then `docker compose restart seaweedfs-admin` |
| Bucket missing in Filer UI | Run the bucket creation command in step 3 |
