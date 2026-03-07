.PHONY: help openclaw-start openclaw-stop openclaw-setup openclaw-logs openclaw-dashboard openclaw-restart openclaw-fix-perms

OPENCLAW_URL := http://127.0.0.1:18789/__openclaw__/canvas/

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-25s\033[0m %s\n", $$1, $$2}'

# ── OpenClaw ──────────────────────────────────────────────────────────────────

openclaw-setup: ## First-time setup: start, patch bind=lan, restart
	@echo "Starting openclaw for initial config generation..."
	docker compose up -d openclaw
	@echo "Waiting for gateway to start (10s)..."
	@sleep 10
	@echo "Waiting for config to be written (up to 50s)..."
	@for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25; do \
		if docker compose exec openclaw sh -c 'test -f /home/node/.openclaw/openclaw.json || test -f /root/.openclaw/openclaw.json' 2>/dev/null; then break; fi; \
		sleep 2; \
	done
	@if ! docker compose exec openclaw sh -c 'test -f /home/node/.openclaw/openclaw.json || test -f /root/.openclaw/openclaw.json' 2>/dev/null; then \
		echo "Config not created by gateway. Creating minimal config in volume (as root)..."; \
		docker compose run --rm --no-deps --user root -v openclaw-data:/home/node/.openclaw openclaw sh -c ' \
			node -e " \
				const fs = require(\"fs\"); const crypto = require(\"crypto\"); \
				const d = \"/home/node/.openclaw\"; const p = d + \"/openclaw.json\"; \
				fs.mkdirSync(d, { recursive: true }); \
				const token = crypto.randomBytes(24).toString(\"hex\"); \
				const cfg = { gateway: { bind: \"lan\", auth: { mode: \"token\", token } } }; \
				fs.writeFileSync(p, JSON.stringify(cfg, null, 2)); \
				console.log(\"Created\", p, \"Token:\", token); \
			" && chown -R 1000:1000 /home/node/.openclaw'; \
		echo "Restarting openclaw to pick up config..."; \
		docker compose restart openclaw; \
		sleep 8; \
	fi
	@docker compose exec openclaw sh -c 'test -f /home/node/.openclaw/openclaw.json || test -f /root/.openclaw/openclaw.json' || (echo "Config file not found. Check: make openclaw-logs"; exit 1)
	@echo "Patching gateway.bind to 'lan' so the UI is reachable from the host..."
	@docker compose exec openclaw node -e " \
		const fs = require('fs'); \
		const paths = ['/home/node/.openclaw/openclaw.json', '/root/.openclaw/openclaw.json']; \
		const p = paths.find(f => { try { fs.accessSync(f); return true; } catch { return false; } }); \
		if (!p) throw new Error('openclaw.json not found'); \
		const c = JSON.parse(fs.readFileSync(p, 'utf8')); \
		c.gateway = c.gateway || {}; \
		c.gateway.bind = 'lan'; \
		fs.writeFileSync(p, JSON.stringify(c, null, 2)); \
		console.log('bind set to lan at', p); \
	"
	@echo "Restarting openclaw with new config..."
	docker compose restart openclaw
	@sleep 3
	@echo ""
	@echo "OpenClaw UI: $(OPENCLAW_URL)"
	@$(MAKE) openclaw-token

openclaw-start: ## Start openclaw service
	docker compose up -d openclaw
	@echo "OpenClaw UI: $(OPENCLAW_URL)"

openclaw-stop: ## Stop openclaw service
	docker compose stop openclaw

openclaw-restart: ## Restart openclaw service
	docker compose restart openclaw

openclaw-logs: ## Tail openclaw logs
	docker compose logs -f openclaw

openclaw-token: ## Print the gateway auth token
	@docker compose exec openclaw node -e " \
		const fs = require('fs'); \
		const paths = ['/home/node/.openclaw/openclaw.json', '/root/.openclaw/openclaw.json']; \
		const p = paths.find(f => { try { fs.accessSync(f); return true; } catch { return false; } }); \
		const c = p ? JSON.parse(fs.readFileSync(p, 'utf8')) : {}; \
		console.log('Token:', c?.gateway?.auth?.token ?? '(not set yet)'); \
	"

openclaw-dashboard: ## Get a fresh dashboard URL (for pairing)
	docker compose exec openclaw openclaw dashboard --no-open

openclaw-fix-perms: ## Fix volume ownership (run if you see EACCES for canvas/cron)
	docker compose run --rm --no-deps --user root -v openclaw-data:/home/node/.openclaw openclaw sh -c 'chown -R 1000:1000 /home/node/.openclaw'
	@echo "Restart openclaw: make openclaw-restart"

openclaw-reset: ## Remove openclaw volume and start fresh (destructive)
	docker compose stop openclaw
	docker compose rm -f openclaw
	docker volume rm crev_openclaw-data 2>/dev/null || true
	@echo "Volume removed. Run 'make openclaw-setup' to reinitialize."
