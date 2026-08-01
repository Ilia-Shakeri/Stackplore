.PHONY: bootstrap dev verify test test-api test-client test-web build check-locales check-contracts derive-api-client lint typecheck format format-check

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

test-client:
	corepack pnpm run test:client

test-web:
	corepack pnpm run test:web

build:
	corepack pnpm run build

check-locales:
	corepack pnpm run check:locales

check-contracts:
	corepack pnpm run check:contracts

derive-api-client:
	corepack pnpm run derive:api-client

lint:
	corepack pnpm run lint

typecheck:
	corepack pnpm run typecheck

format:
	corepack pnpm run format

format-check:
	corepack pnpm run format:check
