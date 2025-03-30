package core

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"pplx2api/config"
	"pplx2api/logger"
	"pplx2api/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/imroc/req/v3"
)

// OpenAISrteamResponse defines OpenAI's streaming response structure
type OpenAISrteamResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []StreamChoice `json:"choices"`
}

// StreamChoice represents a single choice in OpenAI's streaming response
type StreamChoice struct {
	Index        int         `json:"index"`
	Delta        Delta       `json:"delta"`
	Logprobs     interface{} `json:"logprobs"`
	FinishReason interface{} `json:"finish_reason"`
}

// NoStreamChoice represents a single choice in OpenAI's non-streaming response
type NoStreamChoice struct {
	Index        int         `json:"index"`
	Message      Message     `json:"message"`
	Logprobs     interface{} `json:"logprobs"`
	FinishReason string      `json:"finish_reason"`
}

// Delta structure stores the returned text content
type Delta struct {
	Content string `json:"content"`
	Role    string `json:"role"`
}

// Message represents a message in the conversation
type Message struct {
	Role       string        `json:"role"`
	Content    string        `json:"content"`
	Refusal    interface{}   `json:"refusal"`
	Annotation []interface{} `json:"annotation"`
}

// OpenAIResponse represents OpenAI's non-streaming response
type OpenAIResponse struct {
	ID      string           `json:"id"`
	Object  string           `json:"object"`
	Created int64            `json:"created"`
	Model   string           `json:"model"`
	Choices []NoStreamChoice `json:"choices"`
	Usage   Usage            `json:"usage"`
}

// Usage represents token usage information
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Client represents a Perplexity API client
type Client struct {
	sessionToken string
	client       *req.Client
	Model        string
	Attachments  []string
	OpenSerch    bool
}

// Perplexity API structures
type PerplexityRequest struct {
	Params   PerplexityParams `json:"params"`
	QueryStr string           `json:"query_str"`
}

type PerplexityParams struct {
	Attachments              []string      `json:"attachments"`
	Language                 string        `json:"language"`
	Timezone                 string        `json:"timezone"`
	SearchFocus              string        `json:"search_focus"`
	Sources                  []string      `json:"sources"`
	SearchRecencyFilter      interface{}   `json:"search_recency_filter"`
	FrontendUUID             string        `json:"frontend_uuid"`
	Mode                     string        `json:"mode"`
	ModelPreference          string        `json:"model_preference"`
	IsRelatedQuery           bool          `json:"is_related_query"`
	IsSponsored              bool          `json:"is_sponsored"`
	VisitorID                string        `json:"visitor_id"`
	UserNextauthID           string        `json:"user_nextauth_id"`
	FrontendContextUUID      string        `json:"frontend_context_uuid"`
	PromptSource             string        `json:"prompt_source"`
	QuerySource              string        `json:"query_source"`
	LocalSearchEnabled       bool          `json:"local_search_enabled"`
	BrowserHistorySummary    []interface{} `json:"browser_history_summary"`
	IsIncognito              bool          `json:"is_incognito"`
	UseSchematizedAPI        bool          `json:"use_schematized_api"`
	SendBackTextInStreaming  bool          `json:"send_back_text_in_streaming_api"`
	SupportedBlockUseCases   []string      `json:"supported_block_use_cases"`
	ClientCoordinates        interface{}   `json:"client_coordinates"`
	IsNavSuggestionsDisabled bool          `json:"is_nav_suggestions_disabled"`
	Version                  string        `json:"version"`
}

// Response structures
type PerplexityResponse struct {
	Blocks       []Block `json:"blocks"`
	Status       string  `json:"status"`
	DisplayModel string  `json:"display_model"`
}

type Block struct {
	MarkdownBlock      *MarkdownBlock      `json:"markdown_block,omitempty"`
	ReasoningPlanBlock *ReasoningPlanBlock `json:"reasoning_plan_block,omitempty"`
	WebResultBlock     *WebResultBlock     `json:"web_result_block,omitempty"`
}

type MarkdownBlock struct {
	Chunks []string `json:"chunks"`
}

type ReasoningPlanBlock struct {
	Goals []Goal `json:"goals"`
}

type Goal struct {
	Description string `json:"description"`
}

type WebResultBlock struct {
	WebResults []WebResult `json:"web_results"`
}

type WebResult struct {
	Name    string `json:"name"`
	Snippet string `json:"snippet"`
	URL     string `json:"url"`
}

// NewClient creates a new Perplexity API client
func NewClient(sessionToken string, proxy string, model string, openSerch bool) *Client {
	client := req.C().ImpersonateChrome().SetTimeout(time.Minute * 10)
	client.Transport.SetResponseHeaderTimeout(time.Second * 10)
	if proxy != "" {
		client.SetProxyURL(proxy)
	}

	// Set common headers
	headers := map[string]string{
		"accept-language": "en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7,zh-TW;q=0.6",
		"cache-control":   "no-cache",
		"origin":          "https://www.perplexity.ai",
		"pragma":          "no-cache",
		"priority":        "u=1, i",
		"referer":         "https://www.perplexity.ai/",
	}

	for key, value := range headers {
		client.SetCommonHeader(key, value)
	}

	// Set cookies
	if sessionToken != "" {
		client.SetCommonCookies(&http.Cookie{
			Name:  "__Secure-next-auth.session-token",
			Value: sessionToken,
		})
	}

	// Create client with visitor ID
	c := &Client{
		sessionToken: sessionToken,
		client:       client,
		Model:        model,
		Attachments:  []string{},
		OpenSerch:    openSerch,
	}

	return c
}

// SendMessage sends a message to Perplexity and returns the status and response
func (c *Client) SendMessage(message string, stream bool, is_incognito bool, gc *gin.Context) (int, error) {
	// Create request body
	requestBody := PerplexityRequest{
		Params: PerplexityParams{
			Attachments: c.Attachments,
			Language:    "en-US",
			Timezone:    "America/New_York",
			SearchFocus: "writing",
			Sources:     []string{},
			// SearchFocus:             "internet",
			// Sources:                 []string{"web"},
			SearchRecencyFilter:     nil,
			FrontendUUID:            uuid.New().String(),
			Mode:                    "copilot",
			ModelPreference:         c.Model,
			IsRelatedQuery:          false,
			IsSponsored:             false,
			VisitorID:               uuid.New().String(),
			UserNextauthID:          uuid.New().String(),
			FrontendContextUUID:     uuid.New().String(),
			PromptSource:            "user",
			QuerySource:             "home",
			LocalSearchEnabled:      true,
			BrowserHistorySummary:   []interface{}{},
			IsIncognito:             is_incognito,
			UseSchematizedAPI:       true,
			SendBackTextInStreaming: false,
			SupportedBlockUseCases: []string{
				"answer_modes", "media_items", "knowledge_cards", "inline_place_cards",
				"place_widgets", "finance_widgets", "sports_widgets", "shopping_widgets", "jobs_widgets",
			},
			ClientCoordinates:        nil,
			IsNavSuggestionsDisabled: false,
			Version:                  "2.18",
		},
		QueryStr: message,
	}
	if c.OpenSerch {
		requestBody.Params.SearchFocus = "internet"
		requestBody.Params.Sources = append(requestBody.Params.Sources, "web")
	}
	logger.Info(fmt.Sprintf("Perplexity request body: %v", requestBody))
	// Make the request
	resp, err := c.client.R().DisableAutoReadResponse().
		SetBody(requestBody).
		Post("https://www.perplexity.ai/rest/sse/perplexity_ask")

	if err != nil {
		logger.Error(fmt.Sprintf("Error sending request: %v", err))
		return 500, fmt.Errorf("request failed: %w", err)
	}

	logger.Info(fmt.Sprintf("Perplexity response status code: %d", resp.StatusCode))

	if resp.StatusCode == http.StatusTooManyRequests {
		resp.Body.Close()
		return http.StatusTooManyRequests, fmt.Errorf("rate limit exceeded")
	}

	if resp.StatusCode != http.StatusOK {
		logger.Error(fmt.Sprintf("Unexpected return data: %s", resp.String()))
		resp.Body.Close()
		return resp.StatusCode, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return 200, c.HandleResponse(resp.Body, stream, gc)
}

func (c *Client) HandleResponse(body io.ReadCloser, stream bool, gc *gin.Context) error {
	defer body.Close()
	// Set headers for streaming
	if stream {
		gc.Writer.Header().Set("Content-Type", "text/event-stream")
		gc.Writer.Header().Set("Cache-Control", "no-cache")
		gc.Writer.Header().Set("Connection", "keep-alive")
		gc.Writer.WriteHeader(http.StatusOK)
		gc.Writer.Flush()
	} else {
		gc.Writer.Header().Set("Content-Type", "application/json")
		gc.Writer.Header().Set("Cache-Control", "no-cache")
		gc.Writer.Header().Set("Connection", "keep-alive")
	}

	scanner := bufio.NewScanner(body)
	// 增大缓冲区大小
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	full_text := ""
	responseID := uuid.New().String()
	createdTime := time.Now().Unix()
	inThinking := false
	thinkShown := false
	final := false
	for scanner.Scan() {
		line := scanner.Text()
		// Skip empty lines
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		// Extract the data part
		data := line[6:]
		// logger.Info(fmt.Sprintf("Received data: %s", data))
		// Try to parse as PerplexityResponse
		var response PerplexityResponse
		if err := json.Unmarshal([]byte(data), &response); err != nil {
			logger.Error(fmt.Sprintf("Error parsing JSON: %v", err))
			continue
		}
		// Check for completion and web results
		if response.Status == "COMPLETED" {
			final = true
			// Check for web results
			for _, block := range response.Blocks {
				if block.WebResultBlock != nil && len(block.WebResultBlock.WebResults) > 0 {
					webResultsText := "\n\n---\n"
					for i, result := range block.WebResultBlock.WebResults {
						webResultsText += "\n\n" + utils.SearchShow(i, result.Name, result.URL, result.Snippet)
					}
					full_text += webResultsText

					if stream {
						// Send web results
						openAIResp := &OpenAISrteamResponse{
							ID:      responseID,
							Object:  "chat.completion.chunk",
							Created: createdTime,
							Model:   "claude-3-7-sonnet-20250219",
							Choices: []StreamChoice{
								{
									Index: 0,
									Delta: Delta{
										Content: webResultsText,
									},
									Logprobs:     nil,
									FinishReason: nil,
								},
							},
						}
						jsonBytes, err := json.Marshal(openAIResp)
						if err != nil {
							logger.Error(fmt.Sprintf("Error marshalling JSON: %v", err))
							return err
						}
						jsonBytes = append([]byte("data: "), jsonBytes...)
						jsonBytes = append(jsonBytes, []byte("\n\n")...)
						gc.Writer.Write(jsonBytes)
						gc.Writer.Flush()
					}
				}
			}
			if response.DisplayModel != c.Model {
				res_text := "\n\n---\n"
				res_text += fmt.Sprintf("Display Model: %s\n", config.ModelReverseMapGet(response.DisplayModel, response.DisplayModel))
				full_text += res_text
				if !stream {
					break
				}
				// Send model information
				openAIResp := &OpenAISrteamResponse{
					ID:      responseID,
					Object:  "chat.completion.chunk",
					Created: createdTime,
					Model:   "claude-3-7-sonnet-20250219",
					Choices: []StreamChoice{
						{
							Index: 0,
							Delta: Delta{
								Content: res_text,
							},
							Logprobs:     nil,
							FinishReason: nil,
						},
					},
				}
				jsonBytes, err := json.Marshal(openAIResp)
				if err != nil {
					logger.Error(fmt.Sprintf("Error marshalling JSON: %v", err))
					return err
				}
				// Add data: prefix and newlines
				jsonBytes = append([]byte("data: "), jsonBytes...)
				jsonBytes = append(jsonBytes, []byte("\n\n")...)

				// Send data
				gc.Writer.Write(jsonBytes)
				gc.Writer.Flush()
			}
			break
		}
		if final {
			break
		}
		// Process each block in the response
		for _, block := range response.Blocks {
			// Handle reasoning plan blocks (thinking)
			if block.ReasoningPlanBlock != nil && len(block.ReasoningPlanBlock.Goals) > 0 {
				res_text := ""
				if !inThinking && !thinkShown {
					res_text += "<think>"
					inThinking = true
				}

				for _, goal := range block.ReasoningPlanBlock.Goals {
					if goal.Description != "" && goal.Description != "Beginning analysis" && goal.Description != "Wrapping up analysis" {
						res_text += goal.Description
					}
				}
				full_text += res_text
				if !stream {
					continue
				}
				// Create OpenAI format response for text
				openAIResp := &OpenAISrteamResponse{
					ID:      responseID,
					Object:  "chat.completion.chunk",
					Created: createdTime,
					Model:   "claude-3-7-sonnet-20250219",
					Choices: []StreamChoice{
						{
							Index: 0,
							Delta: Delta{
								Content: res_text,
							},
							Logprobs:     nil,
							FinishReason: nil,
						},
					},
				}
				jsonBytes, err := json.Marshal(openAIResp)
				if err != nil {
					logger.Error(fmt.Sprintf("Error marshalling JSON: %v", err))
					return err
				}
				// Add data: prefix and newlines
				jsonBytes = append([]byte("data: "), jsonBytes...)
				jsonBytes = append(jsonBytes, []byte("\n\n")...)

				// Send data
				gc.Writer.Write(jsonBytes)
				gc.Writer.Flush()
			}
			if block.MarkdownBlock != nil && len(block.MarkdownBlock.Chunks) > 0 {
				res_text := ""
				if inThinking {
					res_text += "</think>\n"
					inThinking = false
					thinkShown = true
				}
				for _, chunk := range block.MarkdownBlock.Chunks {
					if chunk != "" {
						res_text += chunk
					}
				}
				full_text += res_text
				if !stream {
					continue
				}
				// Create OpenAI format response for text
				openAIResp := &OpenAISrteamResponse{
					ID:      responseID,
					Object:  "chat.completion.chunk",
					Created: createdTime,
					Model:   "claude-3-7-sonnet-20250219",
					Choices: []StreamChoice{
						{
							Index: 0,
							Delta: Delta{
								Content: res_text,
							},
							Logprobs:     nil,
							FinishReason: nil,
						},
					}}

				jsonBytes, err := json.Marshal(openAIResp)
				if err != nil {
					logger.Error(fmt.Sprintf("Error marshalling JSON: %v", err))
					return err
				}
				// Add data: prefix and newlines
				jsonBytes = append([]byte("data: "), jsonBytes...)
				jsonBytes = append(jsonBytes, []byte("\n\n")...)
				gc.Writer.Write(jsonBytes)
				gc.Writer.Flush()
			}
		}

	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading response: %w", err)
	}

	if !stream {
		// Create final response with all text for non-streaming mode
		openAIResp := &OpenAIResponse{
			ID:      responseID,
			Object:  "chat.completion",
			Created: createdTime,
			Model:   "claude-3-7-sonnet-20250219",
			Choices: []NoStreamChoice{
				{
					Index: 0,
					Message: Message{
						Role:       "assistant",
						Content:    full_text,
						Refusal:    nil,
						Annotation: []interface{}{},
					},
					Logprobs:     nil,
					FinishReason: "stop",
				},
			},
			Usage: Usage{
				PromptTokens:     0,
				CompletionTokens: len(full_text) / 4, // Rough estimate
				TotalTokens:      len(full_text) / 4,
			},
		}
		jsonBytes, err := json.Marshal(openAIResp)
		if err != nil {
			logger.Error(fmt.Sprintf("Error NoStream marshalling JSON: %v", err))
			return err
		}

		gc.Writer.Write(jsonBytes)
		gc.Writer.Flush()
	} else {
		// Send end marker for streaming mode
		gc.Writer.Write([]byte("data: [DONE]\n\n"))
		gc.Writer.Flush()
	}

	return nil
}

// UploadURLResponse represents the response from the create_upload_url endpoint
type UploadURLResponse struct {
	S3BucketURL string               `json:"s3_bucket_url"`
	S3ObjectURL string               `json:"s3_object_url"`
	Fields      CloudinaryUploadInfo `json:"fields"`
	RateLimited bool                 `json:"rate_limited"`
}

type CloudinaryUploadInfo struct {
	Timestamp         int    `json:"timestamp"`
	UniqueFilename    string `json:"unique_filename"`
	Folder            string `json:"folder"`
	UseFilename       string `json:"use_filename"`
	PublicID          string `json:"public_id"`
	Transformation    string `json:"transformation"`
	Moderation        string `json:"moderation"`
	ResourceType      string `json:"resource_type"`
	APIKey            string `json:"api_key"`
	CloudName         string `json:"cloud_name"`
	Signature         string `json:"signature"`
	AWSAccessKeyId    string `json:"AWSAccessKeyId"`
	Key               string `json:"key"`
	Tagging           string `json:"tagging"`
	Policy            string `json:"policy"`
	Xamzsecuritytoken string `json:"x-amz-security-token"`
	ACL               string `json:"acl"`
}

// UploadFile is a placeholder for file upload functionality
func (c *Client) createUploadURL(filename string, contentType string) (*UploadURLResponse, error) {
	requestBody := map[string]interface{}{
		"filename":     filename,
		"content_type": contentType,
		"source":       "default",
		"file_size":    12000,
		"force_image":  false,
	}
	resp, err := c.client.R().
		SetBody(requestBody).
		Post("https://www.perplexity.ai/rest/uploads/create_upload_url?version=2.18&source=default")
	if err != nil {
		logger.Error(fmt.Sprintf("Error creating upload URL: %v", err))
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		logger.Error(fmt.Sprintf("Image Upload with status code %d: %s", resp.StatusCode, resp.String()))
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	var uploadURLResponse UploadURLResponse
	logger.Info(fmt.Sprintf("Create upload with status code %d: %s", resp.StatusCode, resp.String()))
	if err := json.Unmarshal(resp.Bytes(), &uploadURLResponse); err != nil {
		logger.Error(fmt.Sprintf("Error unmarshalling upload URL response: %v", err))
		return nil, err
	}
	if uploadURLResponse.RateLimited {
		logger.Error("Rate limit exceeded for upload URL")
		return nil, fmt.Errorf("rate limit exceeded")
	}
	return &uploadURLResponse, nil

}

func (c *Client) UploadImage(img_list []string) error {
	logger.Info(fmt.Sprintf("Uploading %d images to Cloudinary", len(img_list)))
	filename := utils.RandomString(5) + ".jpg"
	// Upload images to Cloudinary
	for _, img := range img_list {
		// Create upload URL
		uploadURLResponse, err := c.createUploadURL(filename, "image/jpeg")
		if err != nil {
			logger.Error(fmt.Sprintf("Error creating upload URL: %v", err))
			return err
		}
		logger.Info(fmt.Sprintf("Upload URL response: %v", uploadURLResponse))
		// Upload image to Cloudinary
		err = c.UloadFileToCloudinary(uploadURLResponse.Fields, "img", img, filename)
		if err != nil {
			logger.Error(fmt.Sprintf("Error uploading image: %v", err))
			return err
		}
	}
	return nil
}

func (c *Client) UloadFileToCloudinary(uploadInfo CloudinaryUploadInfo, contentType string, filedata string, filename string) error {
	logger.Info(fmt.Sprintf("filedata: %s ……", filedata[:50]))
	// Add form fields
	logger.Info(fmt.Sprintf("Uploading file %s to Cloudinary", filename))
	var formFields map[string]string
	if contentType == "img" {
		formFields = map[string]string{
			"timestamp":       fmt.Sprintf("%d", uploadInfo.Timestamp),
			"unique_filename": uploadInfo.UniqueFilename,
			"folder":          uploadInfo.Folder,
			"use_filename":    uploadInfo.UseFilename,
			"public_id":       uploadInfo.PublicID,
			"transformation":  uploadInfo.Transformation,
			"moderation":      uploadInfo.Moderation,
			"resource_type":   uploadInfo.ResourceType,
			"api_key":         uploadInfo.APIKey,
			"cloud_name":      uploadInfo.CloudName,
			"signature":       uploadInfo.Signature,
		}
	} else {
		formFields = map[string]string{
			"acl":                  uploadInfo.ACL,
			"Content-Type":         "text/plain",
			"tagging":              uploadInfo.Tagging,
			"key":                  uploadInfo.Key,
			"AWSAccessKeyId":       uploadInfo.AWSAccessKeyId,
			"x-amz-security-token": uploadInfo.Xamzsecuritytoken,
			"policy":               uploadInfo.Policy,
			"signature":            uploadInfo.Signature,
		}
	}
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)
	for key, value := range formFields {
		if err := writer.WriteField(key, value); err != nil {
			logger.Error(fmt.Sprintf("Error writing form field %s: %v", key, err))
			return err
		}
	}

	// Add the file,filedata 是base64编码的字符串
	decodedData, err := base64.StdEncoding.DecodeString(filedata)
	if err != nil {
		logger.Error(fmt.Sprintf("Error decoding base64 data: %v", err))
		return err
	}

	// 创建一个文件部分
	part, err := writer.CreateFormFile("file", filename) // 替换 filename.ext 为实际文件名
	if err != nil {
		logger.Error(fmt.Sprintf("Error creating form file: %v", err))
		return err
	}

	// 将解码后的数据写入文件部分
	if _, err := part.Write(decodedData); err != nil {
		logger.Error(fmt.Sprintf("Error writing file data: %v", err))
		return err
	}
	// Close the writer to finalize the form
	if err := writer.Close(); err != nil {
		logger.Error(fmt.Sprintf("Error closing writer: %v", err))
		return err
	}

	// Create the upload request
	var uploadURL string
	if contentType == "img" {
		uploadURL = fmt.Sprintf("https://api.cloudinary.com/v1_1/%s/image/upload", uploadInfo.CloudName)
	} else {
		uploadURL = "https://ppl-ai-file-upload.s3.amazonaws.com/"
	}

	resp, err := c.client.R().
		SetHeader("Content-Type", writer.FormDataContentType()).
		SetBodyBytes(requestBody.Bytes()).
		Post(uploadURL)

	if err != nil {
		logger.Error(fmt.Sprintf("Error uploading file: %v", err))
		return err
	}
	logger.Info(fmt.Sprintf("Image Upload with status code %d: %s", resp.StatusCode, resp.String()))
	if contentType == "img" {
		var uploadResponse map[string]interface{}
		if err := json.Unmarshal(resp.Bytes(), &uploadResponse); err != nil {
			return err
		}
		c.Attachments = append(c.Attachments, uploadResponse["secure_url"].(string))
	} else {
		c.Attachments = append(c.Attachments, "https://ppl-ai-file-upload.s3.amazonaws.com/"+uploadInfo.Key)
	}
	return nil
}

// SetBigContext is a placeholder for setting context
func (c *Client) UploadText(context string) error {
	logger.Info("Uploading txt to Cloudinary")
	filedata := base64.StdEncoding.EncodeToString([]byte(context))
	filename := utils.RandomString(5) + ".txt"
	// Upload images to Cloudinary
	uploadURLResponse, err := c.createUploadURL(filename, "text/plain")
	if err != nil {
		logger.Error(fmt.Sprintf("Error creating upload URL: %v", err))
		return err
	}
	logger.Info(fmt.Sprintf("Upload URL response: %v", uploadURLResponse))
	// Upload txt to Cloudinary
	err = c.UloadFileToCloudinary(uploadURLResponse.Fields, "txt", filedata, filename)
	if err != nil {
		logger.Error(fmt.Sprintf("Error uploading image: %v", err))
		return err
	}

	return nil
}

func (c *Client) GetNewCookie() (string, error) {
	resp, err := c.client.R().Get("https://www.perplexity.ai/api/auth/session")
	if err != nil {
		logger.Error(fmt.Sprintf("Error getting session cookie: %v", err))
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		logger.Error(fmt.Sprintf("Error getting session cookie: %s", resp.String()))
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "__Secure-next-auth.session-token" {
			return cookie.Value, nil
		}
	}
	return "", fmt.Errorf("session cookie not found")
}
