package geminioai

import (
	"fmt"

	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/vertex"
)

// VertexOutboundTransformer is an OutboundTransformer for Vertex AI's OpenAI-compatible endpoint.
// It uses GCP service account credentials for authentication (via vertex.Executor)
// and routes requests to the Vertex AI OpenAI endpoint:
// https://{REGION}-aiplatform.googleapis.com/v1/projects/{PROJECT_ID}/locations/{REGION}/endpoints/openapi
type VertexOutboundTransformer struct {
	transformer.Outbound

	executor *vertex.Executor
}

// VertexConfig holds configuration for the Vertex AI OpenAI-compatible transformer.
type VertexConfig struct {
	// Region is the Google Cloud region, e.g. "us-central1".
	Region string
	// ProjectID is the Google Cloud project ID.
	ProjectID string
	// JSONData is the service account JSON credential data.
	JSONData string
}

// NewVertexOutboundTransformer creates an OutboundTransformer for Vertex AI's OpenAI-compatible endpoint.
// It uses a service-account JSON key for GCP OAuth2 authentication.
func NewVertexOutboundTransformer(config VertexConfig) (transformer.Outbound, error) {
	if config.Region == "" {
		return nil, fmt.Errorf("region is required for gemini_vertex_openai transformer")
	}

	if config.ProjectID == "" {
		return nil, fmt.Errorf("projectID is required for gemini_vertex_openai transformer")
	}

	if config.JSONData == "" {
		return nil, fmt.Errorf("jsonData (service account credentials) is required for gemini_vertex_openai transformer")
	}

	// Create the Vertex AI executor which handles GCP OAuth2 authentication.
	executor, err := vertex.NewExecutorFromJSON(config.Region, config.ProjectID, config.JSONData)
	if err != nil {
		return nil, fmt.Errorf("failed to create vertex executor: %w", err)
	}

	// Build the Vertex AI OpenAI-compatible base URL.
	// The endpoint pattern is:
	// https://{REGION}-aiplatform.googleapis.com/v1/projects/{PROJECT_ID}/locations/{REGION}/endpoints/openapi
	var vertexBaseURL string
	if config.Region == "global" {
		vertexBaseURL = fmt.Sprintf(
			"https://aiplatform.googleapis.com/v1/projects/%s/locations/global/endpoints/openapi",
			config.ProjectID,
		)
	} else {
		vertexBaseURL = fmt.Sprintf(
			"https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/endpoints/openapi",
			config.Region, config.ProjectID, config.Region,
		)
	}

	// Create the underlying geminioai outbound transformer.
	// We use an empty static key provider because actual auth is handled by vertex.Executor.
	oaiTransformer, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        vertexBaseURL,
		APIKeyProvider: auth.NewStaticKeyProvider(""),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create geminioai outbound transformer: %w", err)
	}

	return &VertexOutboundTransformer{
		Outbound: oaiTransformer,
		executor: executor,
	}, nil
}

// CustomizeExecutor returns the Vertex AI GCP-authenticated executor,
// which replaces the default HTTP executor for this channel.
func (t *VertexOutboundTransformer) CustomizeExecutor(defaultExecutor pipeline.Executor) pipeline.Executor {
	return t.executor
}

// Ensure VertexOutboundTransformer implements the ChannelCustomizedExecutor interface.
var _ pipeline.ChannelCustomizedExecutor = (*VertexOutboundTransformer)(nil)
