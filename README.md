go mod init backend

go get github.com/lib/pq
go get golang.org/x/crypto/bcrypt
go get github.com/golang-jwt/jwt/v5
go get github.com/gin-gonic/gin
go get github.com/joho/godotenv
go get github.com/gin-contrib/cors



go mod tidy
cd src
go run . 
