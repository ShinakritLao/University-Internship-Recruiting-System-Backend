go mod init backend

go get github.com/lib/pq
go get golang.org/x/crypto/bcrypt
go get github.com/golang-jwt/jwt/v5
go get github.com/gin-gonic/gin
go get github.com/joho/godotenv
go get github.com/gin-contrib/cors



go mod tidy
go run .

## CI/CD with GitHub Actions and Render

This repo now includes a Dockerfile and a GitHub Actions workflow at `.github/workflows/ci-cd.yml`.

- `Dockerfile` builds the Go app from `./src`.
- GitHub Actions runs `go test ./...` on PRs and pushes a Docker image on `main`.
- The image is published to GitHub Container Registry as `ghcr.io/<owner>/<repo>:latest`.

### Render deployment

To deploy on Render using Docker:
1. Create a Render Web Service with environment `Docker`
2. Point it to this repository and enable auto-deploy from `main`
3. Add your environment variables in Render, including `DB_URL`
4. Optionally set the secret `RENDER_DEPLOY_HOOK` in GitHub Actions to trigger Render after a successful image push

### Required GitHub secret

- `RENDER_DEPLOY_HOOK` (optional): a Render deploy webhook URL if you want the workflow to trigger a deployment automatically
 