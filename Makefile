.PHONY: bootstrap dev verify test test-api test-web build lint typecheck format format-check

bootstrap:
	node scripts/bootstrap.mjs

dev:
	corepack pnpm run dev

verify:
	corepack pnpm run verify

test:
	corepack pnpm run test

test-api:
	corepack pnpm run test:api

test-web:
	corepack pnpm run test:web

build:
	corepack pnpm run build

lint:
	corepack pnpm run lint

typecheck:
	corepack pnpm run typecheck

format:
	corepack pnpm run format

format-check:
	corepack pnpm run format:check
