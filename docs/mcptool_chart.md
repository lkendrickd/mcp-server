# MCP Tool Design Flowchart

This flowchart outlines the decision-making process for designing MCP (Model Context Protocol) tools based on user problems. It helps in determining the appropriate tool type and necessary features to ensure safety, composability, and efficiency.

```mermaid
flowchart TD
    A[Start: User Problem] --> B{What is the job type?}

    B -->|Find things| C[Design search_X tool]
    B -->|List all| D[Design list_X tool]
    B -->|Fetch thing| F{Have ID already?}
    B -->|Change state| J[Design action_X tool]
    B -->|Compute/Transform| M[Design compute_X tool]
    B -->|Validate/Check| V[Design check_X tool]
    B -->|None of above| Z[Reconsider problem framing]

    C --> E[Return IDs + titles + scores + snippets]
    E --> F

    D --> D2[Return paginated list of IDs + summaries]
    D2 --> F

    F -->|No, need discovery| C
    F -->|Yes| G[Design get_X tool]
    G --> H{Is full output large?}

    H -->|Yes| I[Design get_X_excerpt / fields tool]
    H -->|No| N[Proceed]

    J --> K[Add guardrails: dry-run, explicit params, idempotency]
    K --> K2{Batch needed?}
    K2 -->|Yes| K3[Design batch_action_X tool]
    K2 -->|No| N

    M --> O[Ensure deterministic outputs]
    O --> N

    V --> V2[Return boolean + reason]
    V2 --> N

    I --> N
    K3 --> N

    N --> Q{Composable & Safe?}
    Q -->|Yes| R[Ship MCP tools]
    Q -->|No| S[Refactor tools]
    Z --> S
```
