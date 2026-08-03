# Email templates

Operational and lifecycle emails for AgentMesh customers.

## Provider

Use your transactional provider (SendGrid, Postmark, SES). Templates below are content-only.

## Template: `payment_failed`

**Subject:** Action required: AgentMesh payment failed

**Body:**

Hi {{name}},

We could not process your latest payment for org **{{org_id}}**. Update your billing method within 3 days to avoid routing throttling.

[Update billing]({{billing_url}})

## Template: `circuit_opened`

**Subject:** Agent {{agent_id}} circuit opened (org {{org_id}})

**Body:**

The circuit breaker opened for agent **{{agent_id}}** after {{failure_count}} failures in {{window_seconds}}s. Traffic is failing over to: {{fallback_list}}.

Dashboard: {{dashboard_url}}

## Template: `welcome`

**Subject:** Welcome to AgentMesh

**Body:**

Your org **{{org_id}}** is ready. API base: `{{api_base}}`

Quick start: register an agent with `POST /v1/agents/register` and Bearer auth.

## Template: `usage_threshold`

**Subject:** 80% of included routes used

**Body:**

Org **{{org_id}}** has consumed {{percent}}% of monthly included routes. Upgrade or enable overage billing.

## Template: `api_key_rotated`

**Subject:** API key rotated for {{org_id}}

**Body:**

Key ID **{{kid}}** was rotated. Old key expires in {{grace_hours}} hours.

## Compliance

- Include physical mailing address in footer
- Unsubscribe only for marketing; transactional emails are service-critical

## Triggers

Wire triggers in billing worker and circuit event handler ([04-EVENTS.md](04-EVENTS.md)).

## Localization

Default EN-US; store templates per locale under `ops/email/` when i18n ships.
