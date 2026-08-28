package mcpserver

import (
	"context"
	"errors"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"ntdsk.com/mcp/vision/internal/vision"
)

type Server struct {
	client *vision.Client
}

func New(client *vision.Client) *Server {
	return &Server{client: client}
}

func (s *Server) Register(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "vision_image_analysis",
		Description: "General-purpose image understanding with an OpenAI-compatible vision model. Receives an English prompt and the local path or URL (http/https) of an image. Use when no other, more specific tool applies.",
	}, s.imageAnalysis)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "vision_diff",
		Description: "Compares two UI screenshots and flags visual or implementation drift: layout, spacing, colors, missing or extra elements, alignment and typography. Receives an optional English prompt, plus the local path or URL of image A (expected) and image B (actual).",
	}, s.diff)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "vision_video_analysis",
		Description: "Inspects a short video (local or remote, <= 8MB; MP4/MOV/M4V) and describes scenes, moments and entities. The video is sent directly to the vision model. Receives an optional English prompt and the local path or URL of the video.",
	}, s.videoAnalysis)
}

type imageAnalysisInput struct {
	ImagePath string `json:"image_path" jsonschema:"Local path or URL (http/https) of the image to analyse"`
	Prompt    string `json:"prompt" jsonschema:"English instruction for the vision model"`
}

func (s *Server) imageAnalysis(ctx context.Context, _ *mcp.CallToolRequest, in imageAnalysisInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.ImagePath) == "" {
		return nil, nil, errors.New("image_path is required")
	}
	if strings.TrimSpace(in.Prompt) == "" {
		return nil, nil, errors.New("prompt is required")
	}
	img, err := vision.LoadImageBytes(ctx, in.ImagePath, s.client.MaxImageBytes())
	if err != nil {
		return nil, nil, err
	}
	out, err := s.client.Complete(ctx, in.Prompt, []vision.MediaInput{img})
	if err != nil {
		return nil, nil, err
	}
	return textResult(out)
}

type diffInput struct {
	ImageAPath string  `json:"image_a_path" jsonschema:"Local path or URL of the first image (expected/baseline)"`
	ImageBPath string  `json:"image_b_path" jsonschema:"Local path or URL of the second image (actual/obtained)"`
	Prompt     *string `json:"prompt,omitempty" jsonschema:"Optional English instruction; if omitted, compares for visual/implementation drift"`
}

func (s *Server) diff(ctx context.Context, _ *mcp.CallToolRequest, in diffInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.ImageAPath) == "" {
		return nil, nil, errors.New("image_a_path is required")
	}
	if strings.TrimSpace(in.ImageBPath) == "" {
		return nil, nil, errors.New("image_b_path is required")
	}
	imgA, err := vision.LoadImageBytes(ctx, in.ImageAPath, s.client.MaxImageBytes())
	if err != nil {
		return nil, nil, err
	}
	imgB, err := vision.LoadImageBytes(ctx, in.ImageBPath, s.client.MaxImageBytes())
	if err != nil {
		return nil, nil, err
	}

	prompt := defaultDiffPrompt
	if strings.TrimSpace(stringPtr(in.Prompt)) != "" {
		prompt = strings.TrimSpace(stringPtr(in.Prompt))
	}
	prompt += "\n\n(The FIRST image is A / expected; the SECOND image is B / current.)"

	out, err := s.client.Complete(ctx, prompt, []vision.MediaInput{imgA, imgB})
	if err != nil {
		return nil, nil, err
	}
	return textResult(out)
}

type videoAnalysisInput struct {
	VideoPath string  `json:"video_path" jsonschema:"Local path or URL (http/https) of the video (<= 8MB; MP4/MOV/M4V)"`
	Prompt    *string `json:"prompt,omitempty" jsonschema:"Optional English instruction; if omitted, describes scenes/moments/entities"`
}

func (s *Server) videoAnalysis(ctx context.Context, _ *mcp.CallToolRequest, in videoAnalysisInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.VideoPath) == "" {
		return nil, nil, errors.New("video_path is required")
	}
	if !vision.ValidVideoExt(in.VideoPath) {
		return nil, nil, errors.New("unsupported video format; use MP4, MOV or M4V")
	}
	media, cleanup, err := vision.LoadVideoMedia(ctx, in.VideoPath, vision.MaxVideoBytes)
	if err != nil {
		return nil, nil, err
	}
	defer cleanup()

	prompt := defaultVideoPrompt
	if strings.TrimSpace(stringPtr(in.Prompt)) != "" {
		prompt = strings.TrimSpace(stringPtr(in.Prompt))
	}

	out, err := s.client.Complete(ctx, prompt, []vision.MediaInput{media})
	if err != nil {
		return nil, nil, err
	}
	return textResult(out)
}

func stringPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func textResult(text string) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, nil, nil
}
