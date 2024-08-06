# Build stage
FROM golang:1.22.3 as build
WORKDIR /app
COPY . .
# Explicitly set GOOS and GOARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o main cmd/lambda/main.go

# Final stage
FROM public.ecr.aws/lambda/go:1

# Copy the compiled binary from the build stage
COPY --from=build /app/main ${LAMBDA_TASK_ROOT}

# Set the CMD to your handler
CMD ["main"]