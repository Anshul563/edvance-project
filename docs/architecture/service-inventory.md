# Service Inventory

This inventory reflects the services that currently exist in the repository and was verified against the actual directories, module definitions, and configuration files.

## Core Go services

| Service | Language | Directory | Port / config source | Ownership / role | Status |
| --- | --- | --- | --- | --- | --- |
| Admin API | Go | `services/admin-api` | default `8098`, `ADMIN_API_PORT` | Internal admin surface; JWT-authenticated operations | Bootstrap complete; base endpoints and guarded auth in place |
| API Gateway | Go | `services/api-gateway` | default `8080`, `PORT` | Public entrypoint; reverse-proxy and route aggregation | Active gateway with service mounts |
| Auth Service | Go | `services/auth-service` | default `8081`, `AUTH_SERVICE_PORT` / env config | Users, sessions, JWT, email verification | Active |
| User Service | Go | `services/user-service` | default `8082` | User profiles and account data | Present |
| Creator Service | Go | `services/creator-service` | default `8083` | Creator profiles and content ownership | Present |
| Content Service | Go | `services/content-service` | default `8084` | Content entities and media references | Present |
| Video Service | Go | `services/video-service` | default `8085` | Video/media metadata and ingestion workflow | Present |
| Course Service | Go | `services/course-service` | default `8086` | Course, section, and lesson ownership | Present |
| Learning Service | Go | `services/learning-service` | default `8087` | Enrollments, progress, learning state | Present |
| Social Service | Go | `services/social-service` | default `8088` | Social interactions and reputation data | Present |
| Commerce Service | Go | `services/commerce-service` | default `8089` | Carts, coupons, orders, commerce state | Present |
| Payment Service | Go | `services/payment-service` | default `8090` | Payment and webhook processing | Present |
| Notification Service | Go | `services/notification-service` | default `8092` | Event-driven notifications | Present |
| Search Service | Go | `services/search-service` | default `8091` | Discovery/indexing and search APIs | Present |
| Recommendation Service | Python | `services/recommendation-service` | env `RECOMMENDATION_SERVICE_PORT`, default `8093` | Personalized ranking and recommendation events | Implemented and tested |
| Moderation Service | Python | `services/moderation-service` | env `MODERATION_SERVICE_PORT`, default `8094` | Text and content moderation flows | Implemented and tested |
| AI Service | Python | `services/ai-service` | env `AI_SERVICE_PORT`, default `8095` | Tutor/quiz/summary provider calls | Present, backed by mock or configured provider |
| Live Streaming Service | Go | `services/live-streaming-service` | default `8096` | Live session and stream orchestration | Present |
| Analytics Service | Go | `services/analytics-service` | default `8097` | Event ingestion and overview views | Present and tested |
| Analytics Processor | Python | `services/analytics-processor` | not yet wired into the root config | Background event processing | Present but not fully integrated into root orchestration |

## Cross-service ownership notes

- User identity and auth are owned by the Auth Service and validated through JWT claims.
- Course data is owned by the Course Service; downstream services should not write to course tables directly.
- Creator metadata is owned by the Creator Service.
- Learning progress and enrollments are owned by the Learning Service.
- Payments and commerce domains remain separate from learning ownership.
- Admin operations are intentionally fail-closed and restricted to explicit admin user IDs.

## Local purchase profile

The Compose `purchase` profile builds and starts Course, Learning, and Commerce Services; `payment` adds Payment Service and requires Razorpay test credentials. Each service owns its database. Commerce reads current saleable prices from Course Service, Payment calls Commerce over authenticated internal routes, and Commerce provisions Learning through its authenticated internal enrollment route. No current purchase-path service consumes NATS, so Compose does not start a broker.

Checkout and refund retries use caller-provided idempotency keys. Confirmed refunds update Commerce order/purchase state; Learning enrollment is retained by policy and is not automatically revoked. If Learning provisioning fails after payment, Commerce keeps the purchase and returns a retryable failure to Payment.

## Verified gaps and risks

- Docker Compose configuration parses, but container image builds, database migrations, service health checks, and runtime connectivity remain unverified when the Docker daemon is unavailable.
- The default Compose services do not represent every service in the repository. The purchase stack is an optional profile; AI, analytics processing, and the remaining Go services are not included in that purchase workflow.
- Course Service currently has no global catalog-list endpoint; discovery is limited to creator-scoped listing and course detail.
- Refund timeout outcomes are retained as pending and require same-key replay/provider webhook reconciliation; the service will not blindly repeat an uncertain provider operation.

## Health endpoints

The repository convention is consistent across Go services: `GET /health` is the standard application liveness endpoint, while the gateway exposes its own health route. Python services also expose `/health` via FastAPI.

## Implementation status

The repository is a real multi-service system with several active implementations, but not every service is equally complete or equally integrated. The working evidence in this repo supports the following:

- Trusted: gateway, auth, admin API bootstrap, recommendation, moderation, analytics, and major Go services.
- Partially integrated: AI service, analytics processor, and live-streaming configuration.
- Not yet fully operational as a single compose stack: infrastructure automation and root environment bootstrapping.
