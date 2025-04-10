# Design Document: `escuse-me ilm` Subcommands

**Date:** 2025-04-10

## 1. Introduction and Context

Elasticsearch's Index Lifecycle Management (ILM) is a crucial feature for managing time-series data (like logs, metrics, events) efficiently. As indices grow over time, ILM allows automating tasks such as rolling over to new indices, optimizing older indices (shrinking, force merging), moving data between hardware tiers (hot/warm/cold), and eventually deleting data based on retention policies.

Currently, managing ILM often requires direct interaction with the Elasticsearch REST API or using the Kibana UI. The goal of adding `ilm` subcommands to `escuse-me` is to provide a convenient, scriptable, and consistent command-line interface for common ILM tasks, integrating seamlessly with the existing `escuse-me` framework (including connection handling via layers and output formatting via Glazed).

This document outlines the proposed commands and provides context for a developer to implement them.

## 2. ILM Overview (Elasticsearch 8.x)

- **Policies:** The core of ILM. A policy defines a set of phases and actions.
- **Phases:** Represent stages in an index's life (e.g., `hot`, `warm`, `cold`, `frozen`, `delete`). An index moves sequentially through the phases defined in its policy.
- **Actions:** Operations performed within a phase (e.g., `rollover`, `shrink`, `forcemerge`, `freeze`, `set_priority`, `delete`).
- **Triggers:** Conditions that cause an action to execute or a phase transition to occur (e.g., `min_age`, `min_size`, `min_docs`).
- **Application:** ILM policies are typically applied to _new_ indices via index templates. An index template matching a new index can specify the `index.lifecycle.name` (the policy to apply) and often `index.lifecycle.rollover_alias` (needed for the `rollover` action).
- **Management:** Can be managed via Kibana UI or the `/_ilm` REST API endpoints.

**Key Resources:**

- [Elasticsearch ILM Documentation](https://www.elastic.co/guide/en/elasticsearch/reference/current/index-lifecycle-management.html)
- [Logz.io ILM Blog Post](https://logz.io/blog/managing-elasticsearch-indices/)

## 3. Proposed `escuse-me ilm` Commands

This section details the proposed subcommands under `escuse-me ilm`.

### 3.1 Policy Management

**3.1.1 `escuse-me ilm list-policies`**

- **Purpose:** List all defined ILM policies in the cluster.
- **CLI:** `escuse-me ilm list-policies [glazed flags]`
- **ES API:** `GET /_ilm/policy`
- **Output:** Table (via Glazed) showing policy names and potentially version/modified date.

**3.1.2 `escuse-me ilm get-policy`**

- **Purpose:** Show the full JSON definition of one or more specific ILM policies.
- **CLI:** `escuse-me ilm get-policy <policy_name> [policy_name...] [glazed flags]`
- **ES API:** `GET /_ilm/policy/<policy_name>` (potentially multiple calls or `GET /_ilm/policy/<policy1>,<policy2>...`)
- **Output:** JSON output (possibly formatted via Glazed) of the policy definition(s).

**3.1.3 `escuse-me ilm create-policy`**

- **Purpose:** Create a new ILM policy or update an existing one from a JSON definition.
- **CLI:** `escuse-me ilm create-policy --name <policy_name> --file <policy.json>`
- **ES API:** `PUT /_ilm/policy/<policy_name>` (with the JSON content from the file as the request body)
- **Output:** Confirmation message.

**3.1.4 `escuse-me ilm delete-policy`**

- **Purpose:** Delete one or more ILM policies.
- **CLI:** `escuse-me ilm delete-policy <policy_name> [policy_name...]`
- **ES API:** `DELETE /_ilm/policy/<policy_name>` (one call per policy)
- **Output:** Confirmation message(s).

**3.1.5 `escuse-me ilm validate-policy` (Optional/Advanced)**

- **Purpose:** Perform client-side validation of a policy JSON file _before_ sending it to Elasticsearch. Could check for basic structure, known required fields, etc.
- **CLI:** `escuse-me ilm validate-policy --file <policy.json>`
- **ES API:** None (client-side logic).
- **Output:** Validation success message or list of errors/warnings.

### 3.2 Index & Status Management

**3.2.1 `escuse-me ilm explain`**

- **Purpose:** Show the detailed ILM status for one or more indices.
- **CLI:** `escuse-me ilm explain <index_name> [index_name...] [glazed flags]`
- **ES API:** `GET /<index_name>/_ilm/explain` (one call per index, potentially parallelized)
- **Output:** Table (via Glazed) showing index name, managed status, policy, phase, action, step, step info/error.

**3.2.2 `escuse-me ilm list-managed-indices` (Advanced)**

- **Purpose:** List indices that are currently managed by ILM, optionally filtering by policy.
- **CLI:** `escuse-me ilm list-managed-indices [--policy <policy_name>] [glazed flags]`
- **ES API:** Might require `GET /_cat/indices?h=index,settings.index.lifecycle.name` and filtering, or iterating `GET */_ilm/explain` and filtering (potentially slow).
- **Output:** Table (via Glazed) of index names managed by ILM (and their policy if not filtered).

**3.2.3 `escuse-me ilm retry`**

- **Purpose:** Force ILM to retry the current step for an index where ILM processing has failed or stalled.
- **CLI:** `escuse-me ilm retry <index_name> [index_name...]`
- **ES API:** `POST /<index_name>/_ilm/retry` (one call per index)
- **Output:** Confirmation message(s).

### 3.3 Template & Rollover Helpers

**3.3.1 `escuse-me templates add-ilm`**

- **Purpose:** Add or update ILM configuration (`index.lifecycle.name`, `index.lifecycle.rollover_alias`) within an _existing_ index template's settings.
- **CLI:** `escuse-me templates add-ilm --template <template_name> --policy <policy_name> --rollover-alias <alias_name>`
- **ES API:**
  1.  `GET /_index_template/<template_name>`
  2.  Modify the `settings` section of the template JSON.
  3.  `PUT /_index_template/<template_name>` (with the modified template definition)
- **Output:** Confirmation message.
- **Note:** This should probably live under an `escuse-me templates` subcommand group rather than `ilm`.

**3.3.2 `escuse-me ilm force-rollover`**

- **Purpose:** Manually trigger the rollover action for an alias managed by ILM.
- **CLI:** `escuse-me ilm force-rollover <rollover_alias> [--dry-run] [--conditions <json_conditions>]`
- **ES API:** `POST /<rollover_alias>/_rollover` (potentially with conditions in the body)
- **Output:** Result of the rollover attempt.

## 4. Implementation Notes

- **Structure:** Create a new command group under `escuse-me/cmd/escuse-me/cmds/ilm/`. Each command will be a separate Go file implementing the `cobra.Command` and likely the `glazed.GlazeCommand` interface.
- **ES Client:** Utilize the existing `es_layers.NewESClientFromParsedLayers` to get an Elasticsearch client configured via command-line flags/layers.
- **API Calls:** Use the official `elasticsearch-go` client library for making API requests.
- **Error Handling:** Wrap errors using `github.com/pkg/errors`. Parse Elasticsearch error responses gracefully (using helpers if available, like in `clone.go`).
- **Output:** Use the `glazed` processor (`gp middlewares.Processor`) to output data in various formats (table, JSON, YAML).
- **JSON Handling:** Use standard `encoding/json` for parsing policy files and API responses.
- **Concurrency:** For commands operating on multiple indices (like `explain`, `retry`), consider using `errgroup` for concurrent API calls to improve performance.
