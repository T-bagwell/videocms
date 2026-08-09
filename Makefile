.PHONY: db server frontend demo build serve

db:
	createdb videocms 2>/dev/null || true

server:
	cd backend && go run ./cmd/server

frontend:
	cd frontend && npm run dev

demo:
	./scripts/make-demo-media.sh

build:
	cd backend && go build -o bin/videocms-server ./cmd/server
	cd frontend && npm run build

serve: build
	cd backend && ./bin/videocms-server
