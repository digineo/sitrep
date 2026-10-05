# Both build stages run natively and cross-compile for the target platform.
FROM --platform=$BUILDPLATFORM node:24-alpine AS frontend
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY locales /src/locales
COPY frontend ./
RUN npx vite build

FROM --platform=$BUILDPLATFORM golang:1.27 AS backend
ARG TARGETOS TARGETARCH
# The release tag, if any, and the commit hash; .git is not in the context.
ARG VERSION COMMIT
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /src/frontend/dist/app frontend/dist/app
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
		-ldflags "-s -w \
		          -X github.com/digineo/sitrep/internal/buildinfo.Version=$VERSION \
		          -X github.com/digineo/sitrep/internal/buildinfo.Commit=$COMMIT \
		          -X github.com/digineo/sitrep/internal/buildinfo.Date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
		-o /sitrep ./cmd/sitrep \
	&& mkdir /data

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=backend /sitrep /usr/local/bin/sitrep
# Owned by the nonroot user, so that a fresh named volume is writable.
COPY --from=backend --chown=65532:65532 /data /data
# The database and any .env files live in the working directory.
WORKDIR /data
VOLUME /data
EXPOSE 2607
HEALTHCHECK CMD ["/usr/local/bin/sitrep", "healthcheck"]
ENTRYPOINT ["/usr/local/bin/sitrep"]
CMD ["serve"]
