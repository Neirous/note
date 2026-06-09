# Stage 1: Build Vue frontend
FROM node:22-alpine AS frontend
WORKDIR /src
COPY web/frontend/package.json web/frontend/package-lock.json ./
RUN npm ci
COPY web/frontend/ ./
# Vite outputs to ../static (web/static), which resolves to /static in container
RUN npm run build

# Stage 2: Build Go server
FROM golang:1.25-alpine AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Copy built frontend assets to where Go expects them
COPY --from=frontend /static ./web/static
RUN CGO_ENABLED=0 go build -o /app/server ./cmd/server

# Stage 3: Runtime
FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata
COPY --from=backend /app/server /app/server
COPY --from=frontend /static /app/web/static

WORKDIR /app
EXPOSE 8080

ENV APP_ADDR=:8080
ENV APP_DSN=file:/app/data/notes.db?_pragma=busy_timeout(5000)

VOLUME ["/app/data"]

ENTRYPOINT ["/app/server"]
