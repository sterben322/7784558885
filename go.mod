module lastop

go 1.21

require (
    github.com/gin-contrib/cors v1.7.2
    github.com/gin-gonic/gin v1.10.0
    github.com/golang-jwt/jwt/v5 v5.2.1
    github.com/google/uuid v1.6.0
    github.com/joho/godotenv v1.5.1
    github.com/lib/pq v1.10.9
    golang.org/x/crypto v0.31.0
)

replace github.com/gin-gonic/gin => ./stubs/gin
replace github.com/gin-contrib/cors => ./stubs/cors
replace github.com/joho/godotenv => ./stubs/godotenv
replace github.com/google/uuid => ./stubs/uuid
replace github.com/golang-jwt/jwt/v5 => ./stubs/jwtv5
replace github.com/lib/pq => ./stubs/pq
replace golang.org/x/crypto => ./stubs/crypto
