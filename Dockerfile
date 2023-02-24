## build stage
FROM golang:1.19.5-bullseye AS build-stage
WORKDIR /src/
COPY . .

# compile the whole thing without debugging support
RUN go build -ldflags="-s -w" -o /app/webapi ./cmd/webapi

## final stage
FROM debian:bullseye

# expose port 3000, as defined in the SPA
EXPOSE 3000

# copy the compiled binaries
WORKDIR /app/
COPY --from=build-stage /app/webapi ./

# execute the binary
CMD ["/app/webapi"]

# execute with: docker run --name kvasari-api -u 1000:1000 -d -p 3000:3000 kvasari-api
