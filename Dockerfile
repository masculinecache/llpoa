# Stage 1: Build frontend
FROM node:22-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ .
RUN npm run build

# Stage 2: Build Go backend
FROM golang:1.25-alpine AS backend-builder
WORKDIR /app
COPY backend/ .
# Copy the built frontend into the backend's static directory
COPY --from=frontend-builder /app/backend/static ./static
RUN go mod download && CGO_ENABLED=0 go build -o /app/llpoa-server .

# Stage 3: Runtime image
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=backend-builder /app/llpoa-server .
COPY documents/ ./documents/
EXPOSE 8080
CMD ["./llpoa-server"]
