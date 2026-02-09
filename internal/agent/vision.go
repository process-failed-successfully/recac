package agent

import "context"

// VisionAgent extends the Agent interface to support multimodal interactions
type VisionAgent interface {
	Agent
	// SendImage sends a prompt along with an image to the agent and returns the response
	SendImage(ctx context.Context, prompt string, imagePath string) (string, error)
}
