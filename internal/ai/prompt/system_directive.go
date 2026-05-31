package prompt

// SystemDirective is the shared system role prompt sent to LLM providers when
// generating LogicalQuery JSON. It primes the model to emit strict JSON and
// to behave as a Business Intelligence query assistant.
const SystemDirective = "You are a Business Intelligence query assistant. Output only valid JSON."
