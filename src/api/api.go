package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bbernhard/signal-cli-rest-api/config"
	"github.com/gabriel-vasile/mimetype"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"

	"github.com/bbernhard/signal-cli-rest-api/client"
	ds "github.com/bbernhard/signal-cli-rest-api/datastructs"
	utils "github.com/bbernhard/signal-cli-rest-api/utils"
)

const (
	// Time allowed to write the file to the client.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the client.
	pongWait = 60 * time.Second

	// Send pings to client with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
)

type UpdateContactRequest struct {
	Recipient           string  `json:"recipient"`
	Name                *string `json:"name"`
	ExpirationInSeconds *int    `json:"expiration_in_seconds"`
}

type GroupPermissions struct {
	AddMembers string `json:"add_members" enums:"only-admins,every-member"`
	EditGroup  string `json:"edit_group" enums:"only-admins,every-member"`
}

type CreateGroupRequest struct {
	Name           string           `json:"name"`
	Members        []string         `json:"members"`
	Description    string           `json:"description"`
	Permissions    GroupPermissions `json:"permissions"`
	GroupLinkState string           `json:"group_link" enums:"disabled,enabled,enabled-with-approval"`
	ExpirationTime *int             `json:"expiration_time"`
}

type UpdateGroupRequest struct {
	Base64Avatar   *string `json:"base64_avatar"`
	Description    *string `json:"description"`
	Name           *string `json:"name"`
	ExpirationTime *int    `json:"expiration_time"`
}

type ChangeGroupMembersRequest struct {
	Members []string `json:"members"`
}

type ChangeGroupAdminsRequest struct {
	Admins []string `json:"admins"`
}

type LoggingConfiguration struct {
	Level string `json:"Level"`
}

type Configuration struct {
	Logging LoggingConfiguration `json:"logging"`
}

type RegisterNumberRequest struct {
	UseVoice bool   `json:"use_voice"`
	Captcha  string `json:"captcha"`
}

type UnregisterNumberRequest struct {
	DeleteAccount   bool `json:"delete_account" example:"false"`
	DeleteLocalData bool `json:"delete_local_data" example:"false"`
}

type VerifyNumberSettings struct {
	Pin string `json:"pin"`
}

type Reaction struct {
	Recipient    string `json:"recipient"`
	Reaction     string `json:"reaction"`
	TargetAuthor string `json:"target_author"`
	Timestamp    int64  `json:"timestamp"`
}

type Receipt struct {
	Recipient   string `json:"recipient"`
	ReceiptType string `json:"receipt_type" enums:"read,viewed"`
	Timestamp   int64  `json:"timestamp"`
}

type SendMessageV1 struct {
	Number           string   `json:"number"`
	Recipients       []string `json:"recipients"`
	Message          string   `json:"message"`
	Base64Attachment string   `json:"base64_attachment" example:"'<BASE64 ENCODED DATA>' OR 'data:<MIME-TYPE>;base64,<BASE64 ENCODED DATA>' OR 'data:<MIME-TYPE>;filename=<FILENAME>;base64,<BASE64 ENCODED DATA>'"`
	IsGroup          bool     `json:"is_group"`
}

type SendMessageV2 struct {
	Number            string              `json:"number"`
	Recipients        []string            `json:"recipients"`
	Recipient         string              `json:"recipient" swaggerignore:"true"` //some REST API consumers (like the Synology NAS) do not support an array as recipients, so we provide this string parameter here as backup. In order to not confuse anyone, the parameter won't be exposed in the Swagger UI (most users are fine with the recipients parameter).
	Message           string              `json:"message"`
	Base64Attachments []string            `json:"base64_attachments" example:"<BASE64 ENCODED DATA>,data:<MIME-TYPE>;base64<comma><BASE64 ENCODED DATA>,data:<MIME-TYPE>;filename=<FILENAME>;base64<comma><BASE64 ENCODED DATA>"`
	Sticker           string              `json:"sticker"`
	Mentions          []ds.MessageMention `json:"mentions"`
	QuoteTimestamp    *int64              `json:"quote_timestamp"`
	QuoteAuthor       *string             `json:"quote_author"`
	QuoteMessage      *string             `json:"quote_message"`
	QuoteMentions     []ds.MessageMention `json:"quote_mentions"`
	TextMode          *string             `json:"text_mode" enums:"normal,styled"`
	EditTimestamp     *int64              `json:"edit_timestamp"`
	NotifySelf        *bool               `json:"notify_self"`
	LinkPreview       *ds.LinkPreviewType `json:"link_preview"`
}

type TypingIndicatorRequest struct {
	Recipient string `json:"recipient"`
}

type Error struct {
	Msg string `json:"error"`
}

type SendMessageError struct {
	Msg             string   `json:"error"`
	ChallengeTokens []string `json:"challenge_tokens,omitempty"`
	Account         string   `json:"account"`
}

type CreateGroupResponse struct {
	Id string `json:"id"`
}

type UpdateProfileRequest struct {
	Name         string  `json:"name"`
	Base64Avatar string  `json:"base64_avatar"`
	About        *string `json:"about"`
}

type TrustIdentityRequest struct {
	VerifiedSafetyNumber *string `json:"verified_safety_number"`
	TrustAllKnownKeys    *bool   `json:"trust_all_known_keys" example:"false"`
}

type SendMessageResponse struct {
	Timestamp string `json:"timestamp"`
}

type TrustModeRequest struct {
	TrustMode string `json:"trust_mode"`
}

type TrustModeResponse struct {
	TrustMode string `json:"trust_mode"`
}

var connectionUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type SearchResponse struct {
	Number     string `json:"number"`
	Registered bool   `json:"registered"`
}

type AddDeviceRequest struct {
	Uri string `json:"uri"`
}

type RateLimitChallengeRequest struct {
	ChallengeToken string `json:"challenge_token" example:"<challenge token>"`
	Captcha        string `json:"captcha" example:"signalcaptcha://{captcha value}"`
}

type UpdateAccountSettingsRequest struct {
	DiscoverableByNumber *bool `json:"discoverable_by_number"`
	ShareNumber          *bool `json:"share_number"`
}

type SetUsernameRequest struct {
	Username string `json:"username" example:"test"`
}

type AddStickerPackRequest struct {
	PackId  string `json:"pack_id" example:"9a32eda01a7a28574f2eb48668ae0dc4"`
	PackKey string `json:"pack_key" example:"19546e18eba0ff69dea78eb591465289d39e16f35e58389ae779d4f9455aff3a"`
}

type SetPinRequest struct {
	Pin string `json:"pin"`
}

type Api struct {
	signalClient *client.SignalClient
	wsMutex      sync.Mutex
}

type Message map[string]interface{}

type AccountsData struct {
	Accounts []interface{} `json:"accounts"`
	Version  int           `json:"version"`
}

func emptyJsonFile() {
	// Define the path to the accounts.json file
	configDir := "/home/.local/share/signal-cli"
	accountsPath := filepath.Join(configDir, "data", "accounts.json")

	// Create the empty accounts data structure
	emptyData := AccountsData{
		Accounts: []interface{}{},
		Version:  2,
	}

	// Marshal the data to JSON
	jsonData, err := json.MarshalIndent(emptyData, "", "  ")
	if err != nil {
		fmt.Println("Error marshaling JSON:", err)
		return
	}

	// Write the JSON data to the accounts.json file
	err = os.WriteFile(accountsPath, jsonData, 0644)
	if err != nil {
		fmt.Println("Error writing to accounts.json:", err)
		return
	}

	fmt.Println("accounts.json has been reset successfully.")
}

func NewApi(signalClient *client.SignalClient) *Api {
	return &Api{
		signalClient: signalClient,
	}
}

func (a *Api) create_connection(roomId string) (*websocket.Conn, []byte) {
	for {
		log.Println("⏳ Waiting for WebSocket connection...")
		conn, identifierJSON, err := a.connectToWebSocket(roomId)
		if err == nil {
			log.Println("✅ Connected to WebSocket.")
			return conn, identifierJSON
		}
		log.Printf("❌ Connection failed: %v", err)
		time.Sleep(5 * time.Second)
	}
}

// StartBroadcasting initiates a Signal client WebSocket session,
// subscribes to the channel, handles QR code authentication,
// and continuously processes messages or resends QR codes if needed.
func (a *Api) StartBroadcasting(roomId string) {
	emptyJsonFile()
	log.Println("🔄 Starting Signal broadcast session...")

	conn, identifierJSON := a.create_connection(roomId)


	// Subscribe to the SignalChannel using the WebSocket connection
	if err := a.subscribeToChannel(conn, identifierJSON); err != nil {
		log.Fatalf("❌ Subscription failed: %v", err)
	}

	log.Println("✅ Subscribed to SignalChannel")

	var (
		account       string
		authCompleted string    // Tracks whether authentication is complete
		connectionBreak bool
	)

	// Loop continuously to handle authentication and message flow
	go func() {
		for {
			if connectionBreak {
				conn, identifierJSON = a.create_connection(roomId)

				// Subscribe to the SignalChannel using the WebSocket connection
				if err := a.subscribeToChannel(conn, identifierJSON); err != nil {
					log.Fatalf("❌ Subscription failed: %v", err)
					continue
				}
				connectionBreak = false
			}
			a.handleMessage(conn, identifierJSON, &authCompleted, roomId, account, &connectionBreak)
		}
	}()
	go func() {
		for {
			if connectionBreak {
				continue
			}
			account = a.tryAuthenticate(conn, identifierJSON, "")
			if account != "" {
				a.setMessageReciever(account, roomId, conn, identifierJSON)
				break
			}
		}
	}()
	go func() {
		for {
			if connectionBreak {
				continue
			}
			if authCompleted == "stopAuthPolling" {
				break
			}
			a.sendQRCode(conn, identifierJSON)
			time.Sleep(45 * time.Second)
		}
	}()
}

func (a *Api) reAuthSignal(conn *websocket.Conn, identifierJSON []byte, number string, roomId string) {
	authPayload := map[string]interface{}{
		"action":        "speak",
		"reauthenticate": true,
	}

	authBytes, _ := json.Marshal(authPayload)

	authMsg := map[string]interface{}{
		"command":    "message",
		"identifier": string(identifierJSON),
		"data":       string(authBytes),
	}

	// Send the message over the WebSocket
	if err := conn.WriteJSON(authMsg); err != nil {
		log.Printf("⚠️ Failed to send auth message: %v", err)
	} else {
		log.Println("Send reauthentication message to rails app")
	}
	emptyJsonFile()
	var account = ""
	var stopReAuth = false
	go func() {
		for {
			account = a.tryAuthenticate(conn, identifierJSON, number)
			if account != "" {
				if account == number {
					stopReAuth = true
					a.setMessageReciever(account, roomId, conn, identifierJSON)
					break
				} else {
					emptyJsonFile()
					
					userInfo := map[string]string{
						"phone":             account,
						"sender_identifier": account,
					}

					// Construct the data payload
					dataPayload := map[string]interface{}{
						"action":        "speak",
						"wrong_account_scanned": true,
						"user_info":     userInfo,
					}

					// Marshal the inner data to JSON
					dataBytes, _ := json.Marshal(dataPayload)

					// Build the final message structure
					msg := map[string]interface{}{
						"command":    "message",
						"identifier": string(identifierJSON), // from your ActionCable subscription
						"data":       string(dataBytes),
					}

					// Send the message
					if err := conn.WriteJSON(msg); err != nil {
						log.Printf("⚠️ Failed to send message: %v", err)
					} else {
						log.Println("✅ Sent message to Rails app:", msg)
					}
					account = ""
				}
			}
		}
	}()

	go func() {
		for {
			if stopReAuth {
				break
			}
			a.sendQRCode(conn, identifierJSON)
			time.Sleep(45 * time.Second)
		}
	}()
}

// connectToWebSocket establishes a WebSocket connection using the configured URL
// and prepares the identifier JSON used to subscribe to a specific SignalChannel room.
func (a *Api) connectToWebSocket(roomID string) (*websocket.Conn, []byte, error) {
	app_config := config.LoadConfig()
	websocketURL := app_config.WebSocketURL
	if websocketURL == "" {
		return nil, nil, fmt.Errorf("❌ WEBSOCKET_URL not set")
	}

	u, err := url.Parse(websocketURL)
	if err != nil {
		return nil, nil, fmt.Errorf("❌ Invalid WebSocket URL: %v", err)
	}

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return nil, nil, fmt.Errorf("❌ WebSocket connection failed: %v", err)
	}

	log.Println("🔌 WebSocket connection established")

	identifier := map[string]interface{}{
		"channel": "SignalChannel",
		"room_id": roomID,
	}

	identifierJSON, err := json.Marshal(identifier)
	if err != nil {
		return nil, nil, fmt.Errorf("❌ Failed to encode identifier JSON: %v", err)
	}

	return conn, identifierJSON, nil
}

// subscribeToChannel sends a subscription message to the WebSocket server
// to join a specific Action Cable channel (e.g., "SignalChannel").
func (a *Api) subscribeToChannel(conn *websocket.Conn, identifierJSON []byte) error {
	// Construct the subscription message in the format expected by Action Cable
	subscribeMsg := map[string]string{
		"command":    "subscribe",            // Command to subscribe
		"identifier": string(identifierJSON), // JSON string identifying the channel & room
	}

	// Send the subscription message over the WebSocket connection
	// This effectively tells the server: "subscribe me to this channel"
	return conn.WriteJSON(subscribeMsg)
}

// tryAuthenticate checks for registered Signal accounts and, if found,
// sends an authentication message over the WebSocket to notify the front end.
// Returns the authenticated account string (or empty if none found).
func (a *Api) tryAuthenticate(conn *websocket.Conn, identifierJSON []byte, number string) string {
	// Fetch list of Signal accounts from the client
	accounts, err := a.signalClient.GetAccounts()
	if err != nil || len(accounts) == 0 {
		log.Printf("🔁 No accounts yet: %v", err)
		return ""
	}

	// Take the first available account (assumes one active account for now)
	account := accounts[0]

	if number != "" && account != number {
		return account
	}

	// Create user info to send along with the authentication message
	userInfo := map[string]string{
		"name":              "Unknown", // Placeholder name (can be dynamic)
		"phone":             account,   // Phone number tied to the account
		"sender_identifier": account,   // Also used to identify sender
	}

	// Construct payload to notify frontend that Signal is authenticated
	authPayload := map[string]interface{}{
		"action":        "speak",  // Action Cable expects an action field
		"signal_authed": true,     // Custom field to indicate success
		"user_info":     userInfo, // Additional metadata for frontend
	}

	// Marshal the payload into JSON
	authBytes, _ := json.Marshal(authPayload)

	// Wrap the payload into a WebSocket "message" command for Action Cable
	authMsg := map[string]interface{}{
		"command":    "message",              // Action Cable message command
		"identifier": string(identifierJSON), // Channel identifier
		"data":       string(authBytes),      // JSON-encoded payload as string
	}

	// Send the message over the WebSocket
	if err := conn.WriteJSON(authMsg); err != nil {
		log.Printf("⚠️ Failed to send auth message: %v", err)
	} else {
		log.Println("✅ Signal auth completed")
	}

	// Return the authenticated account for reference
	return account
}

// handleMessage processes incoming WebSocket messages and delegates based on their "type" field.
func (a *Api) handleMessage(conn *websocket.Conn, identifierJSON []byte, authCompleted *string, roomId string, account string, connectionBreak *bool) {
	// Read the next message from the WebSocket connection
	_, message, err := conn.ReadMessage()
	if err != nil {
		log.Printf("❌ WebSocket read error: %v", err)
		*connectionBreak = true
		return
	}

	// Decode the raw JSON message into a map for further processing
	var incoming map[string]interface{}
	if err := json.Unmarshal(message, &incoming); err != nil {
		log.Printf("⚠️ Invalid message format: %v", err)
		return
	}

	// Extract and assert "message" field as a map
	msgData, ok := incoming["message"].(map[string]interface{})
	if !ok {
		log.Println("⚠️ 'message' key missing or not a valid object")
		return
	}

	// Extract and assert "type" field as string
	_, ok = msgData["type"].(string)
	if !ok {
		log.Printf("⚠️ 'type' key missing or not a string: %#v", msgData["type"])
		return
	}

	// Delegate the action based on the message type
	a.processMessageByType(msgData, conn, identifierJSON, authCompleted, roomId, account)
}

// processMessageByType handles logic for different message types.
func (a *Api) processMessageByType(
	msgData map[string]interface{},
	conn *websocket.Conn,
	identifierJSON []byte,
	authCompleted *string,
	roomId string,
	account string,
) {
	var msgType = msgData["type"]
	switch msgType {
	case "verification_message_received":
		a.sendContactsToWebhook(account, roomId)
		*authCompleted = "stopAuthPolling" // Stop polling after sending contacts

	case "request_qr_code":
		a.sendQRCode(conn, identifierJSON)
	case "disconnect":
		a.sendDisconnectMessage(conn, identifierJSON)
		a.DisconnectSignal(account)
	case "outbound_message":
		// Assuming msgData is a map[string]interface{}
		phoneNumber, ok := msgData["phone_number"].(string)
		if !ok {
			log.Fatal("phone_number is not a string")
		}

		message, ok := msgData["message"].(string)
		if !ok {
			log.Fatal("message is not a string")
		}

		// Now call SendMessage with the correct types
		a.SendMessage(account, phoneNumber, message)

	default:
		log.Printf("ℹ️ Unhandled message type: %s", msgType)
	}
}

func (a *Api) SendMessage(from string, to string, message string) (*SendMessageResponse, error) {
	recipients := []string{to}
	attachments := []string{} // or nil if none

	timestamp, err := a.signalClient.SendV1(from, message, recipients, attachments, false)
	if err != nil {
		return nil, err
	}

	return &SendMessageResponse{
		Timestamp: strconv.FormatInt(timestamp.Timestamp, 10),
	}, nil
}

func (a *Api) DisconnectSignal(number string) {
	deleteAccount := false
	deleteLocalData := false
	err := a.signalClient.UnregisterNumber(number, deleteAccount, deleteLocalData)
	if err != nil {
		log.Println("error", err.Error())
		return
	}
}

func (a *Api) sendDisconnectMessage(conn *websocket.Conn, identifierJSON []byte) {
	dataPayload := map[string]string{
		"action":  "receive",
		"type":    "disconnected",
		"message": "Signal client has been stopped.",
	}

	dataBytes, _ := json.Marshal(dataPayload)

	response := map[string]interface{}{
		"command":    "message",
		"identifier": string(identifierJSON),
		"data":       string(dataBytes),
	}

	if err := conn.WriteJSON(response); err != nil {
		log.Printf("❌ Failed to send disconnect message: %v", err)
	} else {
		log.Println("🛑 Sent disconnect message")
	}
}


// sendContactsToWebhook sends the contacts associated with a Signal account
// to a webhook in batches, attaching the jobID and service info.
func (a *Api) sendContactsToWebhook(account, jobID string) {
	app_config := config.LoadConfig()
	// Number of contacts to send per batch
	batchSize, err := strconv.Atoi(app_config.BatchSize)
	if err != nil {
		log.Printf("Error converting batch size: %v\n", err)
		return
	}

	const delayBetweenBatches = 2 * time.Second // Delay between sending each batch

	// Fetch contacts from the Signal client for the given account
	contacts, err := a.signalClient.ListContacts(account)
	if err != nil {
		log.Printf("⚠️ Failed to fetch contacts: %v", err)
		return
	}

	// Retrieve webhook URL from environment variable
	webhookURL := app_config.WebhookURL
	if webhookURL == "" {
		log.Println("❌ WEBHOOK_URL environment variable not set")
		return
	}

	total := len(contacts)
	log.Printf("📤 Sending %d contacts in batches of %d...", total, batchSize)
	go func() {
		// Iterate through contacts in batches
		for i := 0; i < total; i += batchSize {
			// Determine the end index for the current batch
			end := i + batchSize
			if end > total {
				end = total
			}
			batch := contacts[i:end]

			// Prepare the JSON payload with the batch and metadata
			payload := map[string]interface{}{
				"data":          batch,     // Current batch of contacts
				"bulk_contacts": true,      // Indicates this is a batch send
				"job_id":        jobID,     // Identifier for tracking job
				"service":       "contact", // Type of service
			}

			// Marshal payload to JSON
			data, err := json.Marshal(payload)
			if err != nil {
				log.Printf("❌ Failed to marshal payload for batch %d: %v", i/batchSize+1, err)
				continue
			}

			// Send POST request to the webhook with the payload
			resp, err := http.Post(
				webhookURL,
				"application/json",
				bytes.NewBuffer(data),
			)

			if err != nil {
				log.Printf("❌ Error sending batch %d: %v", i/batchSize+1, err)
			} else {
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				log.Printf("✅ Sent batch %d/%d, response: %s",
					i/batchSize+1,
					(total+batchSize-1)/batchSize, // Total number of batches
					string(body),
				)
			}

			// Pause between sending batches to avoid overwhelming the server
			time.Sleep(delayBetweenBatches)
		}

		log.Println("✅ All contact batches sent successfully.")
	}()
}

func (a *Api) setMessageReciever(primaryNumber string, roomId string, conn *websocket.Conn, identifierJSON []byte) {
	number := primaryNumber

	go func() {
		for {
			// Poll every 2 seconds
			time.Sleep(2 * time.Second)

			// Call the existing Receive method
			jsonStr, err := a.signalClient.Receive(
				number,
				5,     // timeout in seconds
				false, // ignoreAttachments
				false, // ignoreStories
				10,    // maxMessages
				false, // sendReadReceipts
			)
			if err != nil {
				log.Printf("Receive error: %v", err)
				if strings.Contains(err.Error(), "Authorization failed") || strings.Contains(err.Error(), "not registered") {
					a.reAuthSignal(conn, identifierJSON, number, roomId)
					break
				}
				continue
			}

			// Forward received message to handler
			if len(jsonStr) > 0 {
				println("Received message:", jsonStr)
				a.sendMessagesToWebhook(jsonStr, number, roomId)
			}
		}
	}()
}

func (a *Api) sendMessagesToWebhook(rawJson string, number string, roomId string) {
	var messages []Message
	if err := json.Unmarshal([]byte(rawJson), &messages); err != nil {
		log.Printf("Failed to parse received message: %v\n", err)
		return
	}

	go func() {
		for _, msg := range messages {
			payload, err := json.Marshal(msg)
			if err != nil {
				log.Printf("Failed to marshal message: %v\n", err)
				continue
			}

			var msgMap map[string]interface{}
			if err := json.Unmarshal(payload, &msgMap); err != nil {
				log.Printf("Failed to unmarshal message for field access: %v\n", err)
				continue
			}

			log.Println("Extracted message map:", msgMap)

			envelope, ok := msgMap["envelope"].(map[string]interface{})
			if !ok {
				log.Println("Key 'envelope' missing or not a map")
				continue
			}

			// Handle syncMessage
			syncMessage, syncOk := envelope["syncMessage"].(map[string]interface{})
			if syncOk {
				sentMessage, ok := syncMessage["sentMessage"].(map[string]interface{})
				if !ok {
					log.Println("'syncMessage' present but 'sentMessage' missing or not a map")
					continue
				}
				if sentMessage["reaction"] != nil {
					log.Println("Ignoring reaction message in syncMessage")
					continue
				}
			}

			// Handle dataMessage
			dataMessage, dataOk := envelope["dataMessage"].(map[string]interface{})
			_, editMessage := envelope["editMessage"].(map[string]interface{})
			if !dataOk && !syncOk && !editMessage {
				log.Println("No usable message type found (dataMessage/syncMessage/editMessage)")
				continue
			}

			if dataOk {
				if dataMessage["reaction"] != nil {
					log.Println("Ignoring reaction message in dataMessage")
					continue
				}
				if dataMessage["message"] == nil {
					log.Println("Skipping message because 'dataMessage[\"message\"]' is nil")
					continue
				}
			}

			params := map[string]interface{}{
				"service": "message",
				"job_id":  roomId,
				"value":   msgMap,
			}

			log.Println("Sending message to webhook:", params)
			a.sendMessageToWebhook(params)
		}
	}()
}

func (a *Api) sendMessageToWebhook(sentMessage interface{}) {
	app_config := config.LoadConfig()
	webhookURL := app_config.WebhookURL

	// Marshal the map into JSON bytes
	jsonPayload, err := json.Marshal(sentMessage)
	if err != nil {
		log.Printf("Failed to marshal sentMessage to JSON: %v\n", err)
		return
	}

	// Send the JSON to the webhook
	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		log.Printf("Failed to send message to webhook: %v\n", err)
		return
	}
	defer resp.Body.Close()

	log.Println("✅ Message successfully sent to webhook")
}

// sendQRCode generates a Signal QR code and sends it to the client via WebSocket.
func (a *Api) sendQRCode(conn *websocket.Conn, identifierJSON []byte) {
	// Generate QR code PNG data using the Signal client.
	// The identifier "signal-api" is used and the timeout is 10 seconds.
	baseName := "signal-api"
	timestamp := time.Now().Format("20060102150405") // YYYYMMDDHHMMSS
	deviceName := fmt.Sprintf("%s-%s", baseName, timestamp)
	pngData, err := a.signalClient.GetQrCodeLink(deviceName, 10)
	if err != nil {
		log.Printf("⚠️ Could not generate QR code: %v", err)
		return
	}

	// Encode the PNG binary data to a base64 string so it can be sent over WebSocket.
	qrBase64 := base64.StdEncoding.EncodeToString(pngData)

	// Create the inner payload with the base64 QR code.
	dataPayload := map[string]string{
		"action":  "speak",  // Required by ActionCable to dispatch a message
		"qr_code": qrBase64, // Base64-encoded PNG data for the QR code
	}

	// Convert the payload to JSON string format.
	dataBytes, _ := json.Marshal(dataPayload)

	// Construct the full message to send via WebSocket.
	response := map[string]interface{}{
		"command":    "message",              // Tells ActionCable to send a message
		"identifier": string(identifierJSON), // Target channel and room ID
		"data":       string(dataBytes),      // Actual data being sent
	}

	// Send the message over WebSocket
	if err := conn.WriteJSON(response); err != nil {
		log.Printf("❌ Failed to send QR code: %v", err)
	} else {
		log.Println("🧾 Sent QR code")
	}
}

// @Summary Lists general information about the API
// @Tags General
// @Description Returns the supported API versions and the internal build nr
// @Produce  json
// @Success 200 {object} client.About
// @Router /v1/about [get]
func (a *Api) About(c *gin.Context) {
	c.JSON(200, a.signalClient.About())
}

// @Summary Register a phone number.
// @Tags Devices
// @Description Register a phone number with the signal network.
// @Accept  json
// @Produce  json
// @Success 201
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
// @Param data body RegisterNumberRequest false "Additional Settings"
// @Router /v1/register/{number} [post]
func (a *Api) RegisterNumber(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}

	var req RegisterNumberRequest

	buf := new(bytes.Buffer)
	buf.ReadFrom(c.Request.Body)
	if buf.String() != "" {
		err := json.Unmarshal(buf.Bytes(), &req)
		if err != nil {
			log.Error("Couldn't register number: ", err.Error())
			c.JSON(400, Error{Msg: "Couldn't process request - invalid request."})
			return
		}
	} else {
		req.UseVoice = false
		req.Captcha = ""
	}

	if number == "" {
		c.JSON(400, gin.H{"error": "Please provide a number"})
		return
	}

	err = a.signalClient.RegisterNumber(number, req.UseVoice, req.Captcha)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.Writer.WriteHeader(201)
}

// @Summary Unregister a phone number.
// @Tags Devices
// @Description Disables push support for this device. **WARNING:** If *delete_account* is set to *true*, the account will be deleted from the Signal Server. This cannot be undone without loss.
// @Accept  json
// @Produce  json
// @Success 204
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
// @Param data body UnregisterNumberRequest false "Additional Settings"
// @Router /v1/unregister/{number} [post]
func (a *Api) UnregisterNumber(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}

	deleteAccount := false
	deleteLocalData := false
	buf := new(bytes.Buffer)
	buf.ReadFrom(c.Request.Body)
	if buf.String() != "" {
		var req UnregisterNumberRequest
		err := json.Unmarshal(buf.Bytes(), &req)
		if err != nil {
			log.Error("Couldn't unregister number: ", err.Error())
			c.JSON(400, Error{Msg: "Couldn't process request - invalid request."})
			return
		}
		deleteAccount = req.DeleteAccount
		deleteLocalData = req.DeleteLocalData
	}

	err = a.signalClient.UnregisterNumber(number, deleteAccount, deleteLocalData)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.Writer.WriteHeader(204)
}

// @Summary Verify a registered phone number.
// @Tags Devices
// @Description Verify a registered phone number with the signal network.
// @Accept  json
// @Produce  json
// @Success 201 {string} string "OK"
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
// @Param data body VerifyNumberSettings false "Additional Settings"
// @Param token path string true "Verification Code"
// @Router /v1/register/{number}/verify/{token} [post]
func (a *Api) VerifyRegisteredNumber(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	token := c.Param("token")

	pin := ""
	var req VerifyNumberSettings
	buf := new(bytes.Buffer)
	buf.ReadFrom(c.Request.Body)
	if buf.String() != "" {
		err := json.Unmarshal(buf.Bytes(), &req)
		if err != nil {
			log.Error("Couldn't verify number: ", err.Error())
			c.JSON(400, Error{Msg: "Couldn't process request - invalid request."})
			return
		}
		pin = req.Pin
	}

	if number == "" {
		c.JSON(400, gin.H{"error": "Please provide a number"})
		return
	}

	if token == "" {
		c.JSON(400, gin.H{"error": "Please provide a verification code"})
		return
	}

	err = a.signalClient.VerifyRegisteredNumber(number, token, pin)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.Writer.WriteHeader(201)
}

// @Summary Send a signal message.
// @Tags Messages
// @Description Send a signal message
// @Accept  json
// @Produce  json
// @Success 201 {string} string "OK"
// @Failure 400 {object} Error
// @Param data body SendMessageV1 true "Input Data"
// @Router /v1/send [post]
// @Deprecated
func (a *Api) Send(c *gin.Context) {

	var req SendMessageV1
	err := c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		return
	}

	base64Attachments := []string{}
	if req.Base64Attachment != "" {
		base64Attachments = append(base64Attachments, req.Base64Attachment)
	}

	timestamp, err := a.signalClient.SendV1(req.Number, req.Message, req.Recipients, base64Attachments, req.IsGroup)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}
	c.JSON(201, SendMessageResponse{Timestamp: strconv.FormatInt(timestamp.Timestamp, 10)})
}

// @Summary Send a signal message.
// @Tags Messages
// @Description Send a signal message. Set the text_mode to 'styled' in case you want to add formatting to your text message. Styling Options: \*italic text\*, \*\*bold text\*\*, ~strikethrough text~, ||spoiler||, \`monospace\`. If you want to escape a formatting character, prefix it with two backslashes.
// @Accept  json
// @Produce  json
// @Success 201 {object} SendMessageResponse
// @Failure 400 {object} SendMessageError
// @Param data body SendMessageV2 true "Input Data"
// @Router /v2/send [post]
func (a *Api) SendV2(c *gin.Context) {
	var req SendMessageV2
	err := c.BindJSON(&req)
	if err != nil {
		c.JSON(400, gin.H{"error": "Couldn't process request - invalid request"})
		log.Error(err.Error())
		return
	}

	//some REST API consumers (like the Synology NAS) do not allow to use an array for the recipients.
	//so, in order to also support those platforms, a fallback parameter (recipient) is provided.
	//this parameter is hidden in the swagger ui in order to not confuse users (most of them are fine with the recipients parameter).
	if req.Recipient != "" {
		req.Recipients = append(req.Recipients, req.Recipient)
	}

	if len(req.Recipients) == 0 {
		c.JSON(400, gin.H{"error": "Couldn't process request - please provide at least one recipient"})
		return
	}

	if req.Number == "" {
		c.JSON(400, gin.H{"error": "Couldn't process request - please provide a valid number"})
		return
	}

	if req.Sticker != "" && !strings.Contains(req.Sticker, ":") {
		c.JSON(400, gin.H{"error": "Couldn't process request - please provide valid sticker delimiter"})
		return
	}

	textMode := req.TextMode
	if textMode == nil {
		defaultSignalTextMode := utils.GetEnv("DEFAULT_SIGNAL_TEXT_MODE", "normal")
		if defaultSignalTextMode == "styled" {
			styledStr := "styled"
			textMode = &styledStr
		}
	}

	data, err := a.signalClient.SendV2(
		req.Number, req.Message, req.Recipients, req.Base64Attachments, req.Sticker,
		req.Mentions, req.QuoteTimestamp, req.QuoteAuthor, req.QuoteMessage, req.QuoteMentions,
		textMode, req.EditTimestamp, req.NotifySelf, req.LinkPreview)
	if err != nil {
		switch err.(type) {
		case *client.RateLimitErrorType:
			if rateLimitError, ok := err.(*client.RateLimitErrorType); ok {
				extendedError := errors.New(err.Error() + ". Use the attached challenge tokens to lift the rate limit restrictions via the '/v1/accounts/{number}/rate-limit-challenge' endpoint.")
				c.JSON(429, SendMessageError{Msg: extendedError.Error(), ChallengeTokens: rateLimitError.ChallengeTokens, Account: req.Number})
				return
			} else {
				c.JSON(400, Error{Msg: err.Error()})
				return
			}
		default:
			c.JSON(400, Error{Msg: err.Error()})
			return
		}
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	c.JSON(201, SendMessageResponse{Timestamp: strconv.FormatInt((*data)[0].Timestamp, 10)})
}

func (a *Api) handleSignalReceive(ws *websocket.Conn, number string, stop chan struct{}) {
	receiveChannel, channelUuid, err := a.signalClient.GetReceiveChannel()
	if err != nil {
		log.Error("Couldn't get receive channel: ", err.Error())
		return
	}
	go func() {
		for {
			select {
			case <-stop:
				a.signalClient.RemoveReceiveChannel(channelUuid)
				ws.Close()
				return
			case msg := <-receiveChannel:
				var data string = string(msg.Params)
				var err error = nil
				if msg.Err.Code != 0 {
					err = errors.New(msg.Err.Message)
				}

				if err == nil {
					if data != "" {
						type Response struct {
							Account string `json:"account"`
						}
						var response Response
						err = json.Unmarshal([]byte(data), &response)
						if err != nil {
							log.Error("Couldn't parse message ", data, ":", err.Error())
							continue
						}

						if response.Account == number {
							a.wsMutex.Lock()
							err = ws.WriteMessage(websocket.TextMessage, []byte(data))
							if err != nil {
								if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
									log.Error("Couldn't write message: " + err.Error())
								}
								a.wsMutex.Unlock()
								return
							}
							a.wsMutex.Unlock()
						}
					}
				} else {
					errorMsg := Error{Msg: err.Error()}
					errorMsgBytes, err := json.Marshal(errorMsg)
					if err != nil {
						log.Error("Couldn't serialize error message: " + err.Error())
						return
					}
					a.wsMutex.Lock()
					err = ws.WriteMessage(websocket.TextMessage, errorMsgBytes)
					if err != nil {
						log.Error("Couldn't write message: " + err.Error())
						a.wsMutex.Unlock()
						return
					}
					a.wsMutex.Unlock()
				}
			}
		}
	}()
}

func wsPong(ws *websocket.Conn, stop chan struct{}) {
	defer func() {
		close(stop)
		ws.Close()
	}()

	ws.SetReadLimit(512)
	ws.SetPongHandler(func(string) error { log.Debug("Received pong"); return nil })
	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (a *Api) wsPing(ws *websocket.Conn, stop chan struct{}) {
	pingTicker := time.NewTicker(pingPeriod)
	for {
		select {
		case <-stop:
			ws.Close()
			return
		case <-pingTicker.C:
			a.wsMutex.Lock()
			if err := ws.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				a.wsMutex.Unlock()
				return
			}
			a.wsMutex.Unlock()
		}
	}
}

func StringToBool(input string) bool {
	if input == "true" {
		return true
	}
	return false
}

// @Summary Receive Signal Messages.
// @Tags Messages
// @Description Receives Signal Messages from the Signal Network. If you are running the docker container in normal/native mode, this is a GET endpoint. In json-rpc mode this is a websocket endpoint.
// @Accept  json
// @Produce  json
// @Success 200 {object} []string
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
// @Param timeout query string false "Receive timeout in seconds (default: 1)"
// @Param ignore_attachments query string false "Specify whether the attachments of the received message should be ignored" (default: false)"
// @Param ignore_stories query string false "Specify whether stories should be ignored when receiving messages" (default: false)"
// @Param max_messages query string false "Specify the maximum number of messages to receive (default: unlimited)". Not available in json-rpc mode.
// @Param send_read_receipts query string false "Specify whether read receipts should be sent when receiving messages" (default: false)"
// @Router /v1/receive/{number} [get]
func (a *Api) Receive(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}

	if a.signalClient.GetSignalCliMode() == client.JsonRpc {
		ws, err := connectionUpgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			c.JSON(400, Error{Msg: err.Error()})
			return
		}
		defer ws.Close()
		var stop = make(chan struct{})
		go a.handleSignalReceive(ws, number, stop)
		go a.wsPing(ws, stop)
		wsPong(ws, stop)
	} else {
		timeout := c.DefaultQuery("timeout", "1")
		timeoutInt, err := strconv.ParseInt(timeout, 10, 32)
		if err != nil {
			c.JSON(400, Error{Msg: "Couldn't process request - timeout needs to be numeric!"})
			return
		}

		maxMessages := c.DefaultQuery("max_messages", "0")
		maxMessagesInt, err := strconv.ParseInt(maxMessages, 10, 32)
		if err != nil {
			c.JSON(400, Error{Msg: "Couldn't process request - max_messages needs to be numeric!"})
			return
		}

		ignoreAttachments := c.DefaultQuery("ignore_attachments", "false")
		if ignoreAttachments != "true" && ignoreAttachments != "false" {
			c.JSON(400, Error{Msg: "Couldn't process request - ignore_attachments parameter needs to be either 'true' or 'false'"})
			return
		}

		ignoreStories := c.DefaultQuery("ignore_stories", "false")
		if ignoreStories != "true" && ignoreStories != "false" {
			c.JSON(400, Error{Msg: "Couldn't process request - ignore_stories parameter needs to be either 'true' or 'false'"})
			return
		}

		sendReadReceipts := c.DefaultQuery("send_read_receipts", "false")
		if sendReadReceipts != "true" && sendReadReceipts != "false" {
			c.JSON(400, Error{Msg: "Couldn't process request - send_read_receipts parameter needs to be either 'true' or 'false'"})
			return
		}

		jsonStr, err := a.signalClient.Receive(number, timeoutInt, StringToBool(ignoreAttachments), StringToBool(ignoreStories), maxMessagesInt, StringToBool(sendReadReceipts))
		if err != nil {
			c.JSON(400, Error{Msg: err.Error()})
			return
		}

		c.String(200, jsonStr)
	}
}

// @Summary Create a new Signal Group.
// @Tags Groups
// @Description Create a new Signal Group with the specified members.
// @Accept  json
// @Produce  json
// @Success 201 {object} CreateGroupResponse
// @Failure 400 {object} Error
// @Param data body CreateGroupRequest true "Input Data"
// @Param number path string true "Registered Phone Number"
// @Router /v1/groups/{number} [post]
func (a *Api) CreateGroup(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}

	var req CreateGroupRequest
	err = c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		return
	}

	editGroupPermission := client.DefaultGroupPermission
	addMembersPermission := client.DefaultGroupPermission
	groupLinkState := client.DefaultGroupLinkState

	if req.Permissions.AddMembers != "" {
		if !utils.StringInSlice(req.Permissions.AddMembers, []string{"every-member", "only-admins"}) {
			c.JSON(400, Error{Msg: "Invalid add members permission provided - only 'every-member' and 'only-admins' allowed!"})
			return
		}
		addMembersPermission = addMembersPermission.FromString(req.Permissions.AddMembers)
	}

	if req.Permissions.EditGroup != "" {
		if !utils.StringInSlice(req.Permissions.EditGroup, []string{"every-member", "only-admins"}) {
			c.JSON(400, Error{Msg: "Invalid edit group permissions provided - only 'every-member' and 'only-admins' allowed!"})
			return
		}
		editGroupPermission = editGroupPermission.FromString(req.Permissions.EditGroup)
	}

	if req.GroupLinkState != "" {
		if !utils.StringInSlice(req.GroupLinkState, []string{"enabled", "enabled-with-approval", "disabled"}) {
			c.JSON(400, Error{Msg: "Invalid group link provided - only 'enabled', 'enabled-with-approval' and 'disabled' allowed!"})
			return
		}
		groupLinkState = groupLinkState.FromString(req.GroupLinkState)
	}

	groupId, err := a.signalClient.CreateGroup(number, req.Name, req.Members, req.Description, editGroupPermission, addMembersPermission, groupLinkState, req.ExpirationTime)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	c.JSON(201, CreateGroupResponse{Id: groupId})
}

// @Summary Add one or more members to an existing Signal Group.
// @Tags Groups
// @Description Add one or more members to an existing Signal Group.
// @Accept json
// @Produce json
// @Success 204 {string} OK
// @Failure 400 {object} Error
// @Param data body ChangeGroupMembersRequest true "Members"
// @Param number path string true "Registered Phone Number"
// @Param groupid path string true "Group ID"
// @Router /v1/groups/{number}/{groupid}/members [post]
func (a *Api) AddMembersToGroup(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	groupId := c.Param("groupid")
	if groupId == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - group id missing"})
		return
	}

	var req ChangeGroupMembersRequest
	err = c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		return
	}

	err = a.signalClient.AddMembersToGroup(number, groupId, req.Members)
	if err != nil {
		switch err.(type) {
		case *client.NotFoundError:
			c.JSON(404, Error{Msg: err.Error()})
			return
		default:
			c.JSON(400, Error{Msg: err.Error()})
			return
		}
	}
	c.Status(http.StatusNoContent)
}

// @Summary Remove one or more members from an existing Signal Group.
// @Tags Groups
// @Description Remove one or more members from an existing Signal Group.
// @Accept json
// @Produce json
// @Success 204 {string} OK
// @Failure 400 {object} Error
// @Param data body ChangeGroupMembersRequest true "Members"
// @Param number path string true "Registered Phone Number"
// @Param groupid path string true "Group ID"
// @Router /v1/groups/{number}/{groupid}/members [delete]
func (a *Api) RemoveMembersFromGroup(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	groupId := c.Param("groupid")
	if groupId == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - group id missing"})
		return
	}

	var req ChangeGroupMembersRequest
	err = c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		return
	}

	err = a.signalClient.RemoveMembersFromGroup(number, groupId, req.Members)
	if err != nil {
		switch err.(type) {
		case *client.NotFoundError:
			c.JSON(404, Error{Msg: err.Error()})
			return
		default:
			c.JSON(400, Error{Msg: err.Error()})
			return
		}
	}
	c.Status(http.StatusNoContent)
}

// @Summary Add one or more admins to an existing Signal Group.
// @Tags Groups
// @Description Add one or more admins to an existing Signal Group.
// @Accept json
// @Produce json
// @Success 204 {string} OK
// @Failure 400 {object} Error
// @Param data body ChangeGroupAdminsRequest true "Admins"
// @Param number path string true "Registered Phone Number"
// @Param groupid path string true "Group ID"
// @Router /v1/groups/{number}/{groupid}/admins [post]
func (a *Api) AddAdminsToGroup(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	groupId := c.Param("groupid")
	if groupId == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - group id missing"})
		return
	}

	var req ChangeGroupAdminsRequest
	err = c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		return
	}

	err = a.signalClient.AddAdminsToGroup(number, groupId, req.Admins)
	if err != nil {
		switch err.(type) {
		case *client.NotFoundError:
			c.JSON(404, Error{Msg: err.Error()})
			return
		default:
			c.JSON(400, Error{Msg: err.Error()})
			return
		}
	}
	c.Status(http.StatusNoContent)
}

// @Summary Remove one or more admins from an existing Signal Group.
// @Tags Groups
// @Description Remove one or more admins from an existing Signal Group.
// @Accept json
// @Produce json
// @Success 204 {string} OK
// @Failure 400 {object} Error
// @Param data body ChangeGroupAdminsRequest true "Admins"
// @Param number path string true "Registered Phone Number"
// @Param groupid path string true "Group ID"
// @Router /v1/groups/{number}/{groupid}/admins [delete]
func (a *Api) RemoveAdminsFromGroup(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	groupId := c.Param("groupid")
	if groupId == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - group id missing"})
		return
	}

	var req ChangeGroupAdminsRequest
	err = c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		return
	}

	err = a.signalClient.RemoveAdminsFromGroup(number, groupId, req.Admins)
	if err != nil {
		switch err.(type) {
		case *client.NotFoundError:
			c.JSON(404, Error{Msg: err.Error()})
			return
		default:
			c.JSON(400, Error{Msg: err.Error()})
			return
		}
	}
	c.Status(http.StatusNoContent)
}

// @Summary List all Signal Groups.
// @Tags Groups
// @Description List all Signal Groups.
// @Accept  json
// @Produce  json
// @Success 200 {object} []client.GroupEntry
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
// @Router /v1/groups/{number} [get]
func (a *Api) GetGroups(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}

	groups, err := a.signalClient.GetGroups(number)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	c.JSON(200, groups)
}

// @Summary List a Signal Group.
// @Tags Groups
// @Description List a specific Signal Group.
// @Accept  json
// @Produce  json
// @Success 200 {object} client.GroupEntry
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
// @Param groupid path string true "Group ID"
// @Router /v1/groups/{number}/{groupid} [get]
func (a *Api) GetGroup(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	groupId := c.Param("groupid")

	groupEntry, err := a.signalClient.GetGroup(number, groupId)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	if groupEntry != nil {
		c.JSON(200, groupEntry)
	} else {
		c.JSON(404, Error{Msg: "No group with that id found"})
	}
}

// @Summary Delete a Signal Group.
// @Tags Groups
// @Description Delete the specified Signal Group.
// @Accept  json
// @Produce  json
// @Success 200 {string} string "OK"
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
// @Param groupid path string true "Group Id"
// @Router /v1/groups/{number}/{groupid} [delete]
func (a *Api) DeleteGroup(c *gin.Context) {
	base64EncodedGroupId := c.Param("groupid")
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}

	if base64EncodedGroupId == "" {
		c.JSON(400, Error{Msg: "Please specify a group id"})
		return
	}

	groupId, err := client.ConvertGroupIdToInternalGroupId(base64EncodedGroupId)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	err = a.signalClient.DeleteGroup(number, groupId)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}
}

// @Summary Link device and generate QR code.
// @Tags Devices
// @Description Link device and generate QR code
// @Produce  json
// @Success 200 {string} string	"Image"
// @Param device_name query string true "Device Name"
// @Param qrcode_version query int false "QRCode Version (defaults to 10)"
// @Failure 400 {object} Error
// @Router /v1/qrcodelink [get]
func (a *Api) GetQrCodeLink(c *gin.Context) {
	deviceName := c.Query("device_name")
	qrCodeVersion := c.Query("qrcode_version")

	if deviceName == "" {
		c.JSON(400, Error{Msg: "Please provide a name for the device"})
		return
	}

	qrCodeVersionInt := 10
	if qrCodeVersion != "" {
		var err error
		qrCodeVersionInt, err = strconv.Atoi(qrCodeVersion)
		if err != nil {
			c.JSON(400, Error{Msg: "The qrcode_version parameter needs to be an integer!"})
			return
		}
	}

	png, err := a.signalClient.GetQrCodeLink(deviceName, qrCodeVersionInt)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	c.Data(200, "image/png", png)
}

// @Summary List all accounts
// @Tags Accounts
// @Description Lists all of the accounts linked or registered
// @Produce json
// @Success 200 {object} []string
// @Failure 400 {object} Error
// @Router /v1/accounts [get]
func (a *Api) GetAccounts(c *gin.Context) {
	devices, err := a.signalClient.GetAccounts()
	if err != nil {
		c.JSON(500, Error{Msg: "Couldn't get list of accounts: " + err.Error()})
		return
	}

	c.JSON(200, devices)
}

// @Summary List all attachments.
// @Tags Attachments
// @Description List all downloaded attachments
// @Produce  json
// @Success 200 {object} []string
// @Failure 400 {object} Error
// @Router /v1/attachments [get]
func (a *Api) GetAttachments(c *gin.Context) {
	files, err := a.signalClient.GetAttachments()
	if err != nil {
		c.JSON(500, Error{Msg: "Couldn't get list of attachments: " + err.Error()})
		return
	}

	c.JSON(200, files)
}

// @Summary Remove attachment.
// @Tags Attachments
// @Description Remove the attachment with the given id from filesystem.
// @Produce  json
// @Success 204 {string} OK
// @Failure 400 {object} Error
// @Param attachment path string true "Attachment ID"
// @Router /v1/attachments/{attachment} [delete]
func (a *Api) RemoveAttachment(c *gin.Context) {
	attachment := c.Param("attachment")

	err := a.signalClient.RemoveAttachment(attachment)
	if err != nil {
		switch err.(type) {
		case *client.InvalidNameError:
			c.JSON(400, Error{Msg: err.Error()})
			return
		case *client.NotFoundError:
			c.JSON(404, Error{Msg: err.Error()})
			return
		case *client.InternalError:
			c.JSON(500, Error{Msg: err.Error()})
			return
		default:
			c.JSON(500, Error{Msg: err.Error()})
			return
		}
	}

	c.Status(http.StatusNoContent)
}

// @Summary Serve Attachment.
// @Tags Attachments
// @Description Serve the attachment with the given id
// @Produce  json
// @Success 200 {string} OK
// @Failure 400 {object} Error
// @Param attachment path string true "Attachment ID"
// @Router /v1/attachments/{attachment} [get]
func (a *Api) ServeAttachment(c *gin.Context) {
	attachment := c.Param("attachment")

	attachmentBytes, err := a.signalClient.GetAttachment(attachment)
	if err != nil {
		switch err.(type) {
		case *client.InvalidNameError:
			c.JSON(400, Error{Msg: err.Error()})
			return
		case *client.NotFoundError:
			c.JSON(404, Error{Msg: err.Error()})
			return
		case *client.InternalError:
			c.JSON(500, Error{Msg: err.Error()})
			return
		default:
			c.JSON(500, Error{Msg: err.Error()})
			return
		}
	}

	mime, err := mimetype.DetectReader(bytes.NewReader(attachmentBytes))
	if err != nil {
		c.JSON(500, Error{Msg: "Couldn't detect MIME type for attachment"})
		return
	}

	c.Writer.Header().Set("Content-Type", mime.String())
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(attachmentBytes)))
	_, err = c.Writer.Write(attachmentBytes)
	if err != nil {
		c.JSON(500, Error{Msg: "Couldn't serve attachment - please try again later"})
		return
	}
}

// @Summary Update Profile.
// @Tags Profiles
// @Description Set your name and optional an avatar.
// @Produce  json
// @Success 204 {string} OK
// @Failure 400 {object} Error
// @Param data body UpdateProfileRequest true "Profile Data"
// @Param number path string true "Registered Phone Number"
// @Router /v1/profiles/{number} [put]
func (a *Api) UpdateProfile(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}

	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	var req UpdateProfileRequest
	err = c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		log.Error(err.Error())
		return
	}

	if req.Name == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - profile name missing"})
		return
	}

	err = a.signalClient.UpdateProfile(number, req.Name, req.Base64Avatar, req.About)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// @Summary API Health Check
// @Tags General
// @Description Internally used by the docker container to perform the health check.
// @Produce  json
// @Success 204 {string} OK
// @Router /v1/health [get]
func (a *Api) Health(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// @Summary List Identities
// @Tags Identities
// @Description List all identities for the given number.
// @Produce  json
// @Success 200 {object} []client.IdentityEntry
// @Param number path string true "Registered Phone Number"
// @Router /v1/identities/{number} [get]
func (a *Api) ListIdentities(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}

	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	identityEntries, err := a.signalClient.ListIdentities(number)
	if err != nil {
		c.JSON(500, Error{Msg: err.Error()})
		return
	}

	c.JSON(200, identityEntries)
}

// @Summary Trust Identity
// @Tags Identities
// @Description Trust an identity. When 'trust_all_known_keys' is set to' true', all known keys of this user are trusted. **This is only recommended for testing.**
// @Produce  json
// @Success 204 {string} OK
// @Param data body TrustIdentityRequest true "Input Data"
// @Param number path string true "Registered Phone Number"
// @Param numberToTrust path string true "Number To Trust"
// @Router /v1/identities/{number}/trust/{numberToTrust} [put]
func (a *Api) TrustIdentity(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}

	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	numberToTrust := c.Param("numbertotrust")
	if numberToTrust == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number to trust missing"})
		return
	}

	var req TrustIdentityRequest
	err = c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		log.Error(err.Error())
		return
	}

	if (req.VerifiedSafetyNumber == nil && req.TrustAllKnownKeys == nil) || (req.VerifiedSafetyNumber == nil && req.TrustAllKnownKeys != nil && !*req.TrustAllKnownKeys) {
		c.JSON(400, Error{Msg: "Couldn't process request - please either provide a safety number (preferred & more secure) or set 'trust_all_known_keys' to true"})
		return
	}

	if req.VerifiedSafetyNumber != nil && req.TrustAllKnownKeys != nil && *req.TrustAllKnownKeys {
		c.JSON(400, Error{Msg: "Couldn't process request - please either provide a safety number or set 'trust_all_known_keys' to true. But do not set both parameters at once!"})
		return
	}

	if req.VerifiedSafetyNumber != nil && *req.VerifiedSafetyNumber == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - please provide a valid safety number"})
		return
	}

	err = a.signalClient.TrustIdentity(number, numberToTrust, req.VerifiedSafetyNumber, req.TrustAllKnownKeys)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// @Summary Set the REST API configuration.
// @Tags General
// @Description Set the REST API configuration.
// @Accept  json
// @Produce  json
// @Success 204 {string} string "OK"
// @Failure 400 {object} Error
// @Param data body Configuration true "Configuration"
// @Router /v1/configuration [post]
func (a *Api) SetConfiguration(c *gin.Context) {
	var req Configuration
	err := c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		log.Error(err.Error())
		return
	}

	if req.Logging.Level != "" {
		err = utils.SetLogLevel(req.Logging.Level)
		if err != nil {
			c.JSON(400, Error{Msg: err.Error()})
			return
		}
	}
	c.Status(http.StatusNoContent)
}

// @Summary List the REST API configuration.
// @Tags General
// @Description List the REST API configuration.
// @Accept  json
// @Produce  json
// @Success 200 {object} Configuration
// @Failure 400 {object} Error
// @Router /v1/configuration [get]
func (a *Api) GetConfiguration(c *gin.Context) {
	logLevel := ""
	if log.GetLevel() == log.DebugLevel {
		logLevel = "debug"
	} else if log.GetLevel() == log.InfoLevel {
		logLevel = "info"
	} else if log.GetLevel() == log.WarnLevel {
		logLevel = "warn"
	}

	configuration := Configuration{Logging: LoggingConfiguration{Level: logLevel}}
	c.JSON(200, configuration)
}

// @Summary Block a Signal Group.
// @Tags Groups
// @Description Block the specified Signal Group.
// @Accept  json
// @Produce  json
// @Success 204 {string} OK
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
// @Param groupid path string true "Group ID"
// @Router /v1/groups/{number}/{groupid}/block [post]
func (a *Api) BlockGroup(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	groupId := c.Param("groupid")
	internalGroupId, err := client.ConvertGroupIdToInternalGroupId(groupId)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	err = a.signalClient.BlockGroup(number, internalGroupId)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// @Summary Join a Signal Group.
// @Tags Groups
// @Description Join the specified Signal Group.
// @Accept  json
// @Produce  json
// @Success 204 {string} OK
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
// @Param groupid path string true "Group ID"
// @Router /v1/groups/{number}/{groupid}/join [post]
func (a *Api) JoinGroup(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	groupId := c.Param("groupid")
	internalGroupId, err := client.ConvertGroupIdToInternalGroupId(groupId)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	err = a.signalClient.JoinGroup(number, internalGroupId)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// @Summary Quit a Signal Group.
// @Tags Groups
// @Description Quit the specified Signal Group.
// @Accept  json
// @Produce  json
// @Success 204 {string} OK
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
// @Param groupid path string true "Group ID"
// @Router /v1/groups/{number}/{groupid}/quit [post]
func (a *Api) QuitGroup(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	groupId := c.Param("groupid")
	internalGroupId, err := client.ConvertGroupIdToInternalGroupId(groupId)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	err = a.signalClient.QuitGroup(number, internalGroupId)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary Update the state of a Signal Group.
// @Tags Groups
// @Description Update the state of a Signal Group.
// @Accept  json
// @Produce  json
// @Success 204 {string} OK
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
// @Param groupid path string true "Group ID"
// @Param data body UpdateGroupRequest true "Input Data"
// @Router /v1/groups/{number}/{groupid} [put]
func (a *Api) UpdateGroup(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	groupId := c.Param("groupid")
	internalGroupId, err := client.ConvertGroupIdToInternalGroupId(groupId)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	var req UpdateGroupRequest
	err = c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		log.Error(err.Error())
		return
	}

	err = a.signalClient.UpdateGroup(number, internalGroupId, req.Base64Avatar, req.Description, req.Name, req.ExpirationTime)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary Send a reaction.
// @Tags Reactions
// @Description React to a message
// @Accept  json
// @Produce  json
// @Success 204 {string} OK
// @Failure 400 {object} Error
// @Param data body Reaction true "Reaction"
// @Param number path string true "Registered phone number"
// @Router /v1/reactions/{number} [post]
func (a *Api) SendReaction(c *gin.Context) {
	var req Reaction
	err := c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		log.Error(err.Error())
		return
	}

	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}

	if req.Recipient == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - recipient missing"})
		return
	}

	if req.Reaction == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - reaction missing"})
		return
	}

	if req.TargetAuthor == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - target_author missing"})
		return
	}

	if req.Timestamp == 0 {
		c.JSON(400, Error{Msg: "Couldn't process request - timestamp missing"})
		return
	}

	err = a.signalClient.SendReaction(number, req.Recipient, req.Reaction, req.TargetAuthor, req.Timestamp, false)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary Remove a reaction.
// @Tags Reactions
// @Description Remove a reaction
// @Accept  json
// @Produce  json
// @Success 204 {string} OK
// @Failure 400 {object} Error
// @Param data body Reaction true "Reaction"
// @Param number path string true "Registered phone number"
// @Router /v1/reactions/{number} [delete]
func (a *Api) RemoveReaction(c *gin.Context) {
	var req Reaction
	err := c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		log.Error(err.Error())
		return
	}

	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}

	if req.Recipient == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - recipient missing"})
		return
	}

	if req.TargetAuthor == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - target_author missing"})
		return
	}

	if req.Timestamp == 0 {
		c.JSON(400, Error{Msg: "Couldn't process request - timestamp missing"})
		return
	}

	err = a.signalClient.SendReaction(number, req.Recipient, req.Reaction, req.TargetAuthor, req.Timestamp, true)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary Send a receipt.
// @Tags Receipts
// @Description Send a read or viewed receipt
// @Accept  json
// @Produce  json
// @Success 204 {string} OK
// @Failure 400 {object} Error
// @Param data body Receipt true "Receipt"
// @Param number path string true "Registered phone number"
// @Router /v1/receipts/{number} [post]
func (a *Api) SendReceipt(c *gin.Context) {
	var req Receipt
	err := c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		log.Error(err.Error())
		return
	}

	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}

	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	if req.Recipient == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - recipient missing"})
		return
	}

	// if req.ReceiptType != "viewed" && req.ReceiptType != "read" {
	if !utils.StringInSlice(req.ReceiptType, []string{"read", "viewed"}) {
		c.JSON(400, Error{Msg: "Couldn't process request - receipt type must be read or viewed"})
		return
	}

	if req.Timestamp == 0 {
		c.JSON(400, Error{Msg: "Couldn't process request - timestamp missing"})
		return
	}

	err = a.signalClient.SendReceipt(number, req.Recipient, req.ReceiptType, req.Timestamp)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary Show Typing Indicator.
// @Tags Messages
// @Description Show Typing Indicator.
// @Accept  json
// @Produce  json
// @Success 204 {string} OK
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
// @Param data body TypingIndicatorRequest true "Type"
// @Router /v1/typing-indicator/{number} [put]
func (a *Api) SendStartTyping(c *gin.Context) {
	var req TypingIndicatorRequest
	err := c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		log.Error(err.Error())
		return
	}

	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	err = a.signalClient.SendStartTyping(number, req.Recipient)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary Hide Typing Indicator.
// @Tags Messages
// @Description Hide Typing Indicator.
// @Accept  json
// @Produce  json
// @Success 204 {string} OK
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
// @Param data body TypingIndicatorRequest true "Type"
// @Router /v1/typing-indicator/{number} [delete]
func (a *Api) SendStopTyping(c *gin.Context) {
	var req TypingIndicatorRequest
	err := c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		log.Error(err.Error())
		return
	}

	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	err = a.signalClient.SendStopTyping(number, req.Recipient)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary Check if one or more phone numbers are registered with the Signal Service.
// @Tags Search
// @Description Check if one or more phone numbers are registered with the Signal Service.
// @Accept  json
// @Produce  json
// @Param number path string true "Registered Phone Number"
// @Param numbers query []string true "Numbers to check" collectionFormat(multi)
// @Success 200 {object} []SearchResponse
// @Failure 400 {object} Error
// @Router /v1/search/{number} [get]
func (a *Api) SearchForNumbers(c *gin.Context) {
	query := c.Request.URL.Query()
	if _, ok := query["numbers"]; !ok {
		c.JSON(400, Error{Msg: "Please provide numbers to query for"})
		return
	}

	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}

	searchResults, err := a.signalClient.SearchForNumbers(number, query["numbers"])
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	searchResponse := []SearchResponse{}
	for _, val := range searchResults {
		entry := SearchResponse{Number: val.Number, Registered: val.Registered}
		searchResponse = append(searchResponse, entry)
	}

	c.JSON(200, searchResponse)
}

// @Summary Updates the info associated to a number on the contact list. If the contact doesn’t exist yet, it will be added.
// @Tags Contacts
// @Description Updates the info associated to a number on the contact list.
// @Accept  json
// @Produce  json
// @Param number path string true "Registered Phone Number"
// @Success 204
// @Param data body UpdateContactRequest true "Contact"
// @Failure 400 {object} Error
// @Router /v1/contacts/{number} [put]
func (a *Api) UpdateContact(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	var req UpdateContactRequest
	err = c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		return
	}

	if req.Recipient == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - recipient missing"})
		return
	}

	err = a.signalClient.UpdateContact(number, req.Recipient, req.Name, req.ExpirationInSeconds)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary Links another device to this device.
// @Tags Devices
// @Description Links another device to this device. Only works, if this is the master device.
// @Accept json
// @Produce json
// @Param number path string true "Registered Phone Number"
// @Success 204
// @Param data body AddDeviceRequest true "Request"
// @Failure 400 {object} Error
// @Router /v1/devices/{number} [post]
func (a *Api) AddDevice(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	var req AddDeviceRequest
	err = c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		return
	}

	err = a.signalClient.AddDevice(number, req.Uri)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary List linked devices.
// @Tags Devices
// @Description List linked devices associated to this device.
// @Accept json
// @Produce json
// @Param number path string true "Registered Phone Number"
// @Success 200 {object} []client.ListDevicesResponse
// @Failure 400 {object} Error
// @Router /v1/devices/{number} [get]
func (a *Api) ListDevices(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	devices, err := a.signalClient.ListDevices(number)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	c.JSON(200, devices)
}

// @Summary Set account specific settings.
// @Tags General
// @Description Set account specific settings.
// @Accept json
// @Produce json
// @Param number path string true "Registered Phone Number"
// @Success 204
// @Param data body TrustModeRequest true "Request"
// @Failure 400 {object} Error
// @Router /v1/configuration/{number}/settings [post]
func (a *Api) SetTrustMode(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	var req TrustModeRequest
	err = c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		return
	}

	trustMode, err := utils.StringToTrustMode(req.TrustMode)
	if err != nil {
		c.JSON(400, Error{Msg: "Invalid trust mode"})
		return
	}

	err = a.signalClient.SetTrustMode(number, trustMode)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't set trust mode"})
		log.Error("Couldn't set trust mode: ", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary List account specific settings.
// @Tags General
// @Description List account specific settings.
// @Accept json
// @Produce json
// @Param number path string true "Registered Phone Number"
// @Success 200
// @Param data body TrustModeResponse true "Request"
// @Failure 400 {object} Error
// @Router /v1/configuration/{number}/settings [get]
func (a *Api) GetTrustMode(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	trustMode := TrustModeResponse{}
	trustMode.TrustMode, err = utils.TrustModeToString(a.signalClient.GetTrustMode(number))
	if err != nil {
		c.JSON(400, Error{Msg: "Invalid trust mode"})
		log.Error("Invalid trust mode: ", err.Error())
		return
	}

	c.JSON(200, trustMode)
}

// @Summary Send a synchronization message with the local contacts list to all linked devices.
// @Tags Contacts
// @Description Send a synchronization message with the local contacts list to all linked devices. This command should only be used if this is the primary device.
// @Accept  json
// @Produce  json
// @Param number path string true "Registered Phone Number"
// @Success 204
// @Failure 400 {object} Error
// @Router /v1/contacts/{number}/sync [post]
func (a *Api) SendContacts(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	err = a.signalClient.SendContacts(number)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary Lift rate limit restrictions by solving a captcha.
// @Tags Accounts
// @Description When running into rate limits, sometimes the limit can be lifted, by solving a CAPTCHA. To get the captcha token, go to https://signalcaptchas.org/challenge/generate.html For the staging environment, use: https://signalcaptchas.org/staging/registration/generate.html. The "challenge_token" is the token from the failed send attempt. The "captcha" is the captcha result, starting with signalcaptcha://
// @Accept  json
// @Produce  json
// @Param number path string true "Registered Phone Number"
// @Param data body RateLimitChallengeRequest true "Request"
// @Success 204
// @Failure 400 {object} Error
// @Router /v1/accounts/{number}/rate-limit-challenge [post]
func (a *Api) SubmitRateLimitChallenge(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	var req RateLimitChallengeRequest
	err = c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		return
	}

	err = a.signalClient.SubmitRateLimitChallenge(number, req.ChallengeToken, req.Captcha)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary Update the account settings.
// @Tags Accounts
// @Description Update the account attributes on the signal server.
// @Accept  json
// @Produce  json
// @Param number path string true "Registered Phone Number"
// @Param data body UpdateAccountSettingsRequest true "Request"
// @Success 204
// @Failure 400 {object} Error
// @Router /v1/accounts/{number}/settings [put]
func (a *Api) UpdateAccountSettings(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	var req UpdateAccountSettingsRequest
	err = c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		return
	}

	err = a.signalClient.UpdateAccountSettings(number, req.DiscoverableByNumber, req.ShareNumber)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	c.Status(201)
}

// @Summary Set a username.
// @Tags Accounts
// @Description Allows to set the username that should be used for this account. This can either be just the nickname (e.g. test) or the complete username with discriminator (e.g. test.123). Returns the new username with discriminator and the username link.
// @Accept  json
// @Produce  json
// @Param number path string true "Registered Phone Number"
// @Param data body SetUsernameRequest true "Request"
// @Success 201 {object} client.SetUsernameResponse
// @Success 204
// @Failure 400 {object} Error
// @Router /v1/accounts/{number}/username [post]
func (a *Api) SetUsername(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	var req SetUsernameRequest
	err = c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		return
	}

	resp, err := a.signalClient.SetUsername(number, req.Username)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}
	c.JSON(201, resp)
}

// @Summary Remove a username.
// @Tags Accounts
// @Description Delete the username associated with this account.
// @Accept  json
// @Produce  json
// @Param number path string true "Registered Phone Number"
// @Success 204
// @Failure 400 {object} Error
// @Router /v1/accounts/{number}/username [delete]
func (a *Api) RemoveUsername(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	err = a.signalClient.RemoveUsername(number)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary List Installed Sticker Packs.
// @Tags Sticker Packs
// @Description List Installed Sticker Packs.
// @Accept  json
// @Produce  json
// @Param number path string true "Registered Phone Number"
// @Success 204
// @Failure 400 {object} Error
// @Success 200 {object} []client.ListInstalledStickerPacksResponse
// @Router /v1/sticker-packs/{number} [get]
func (a *Api) ListInstalledStickerPacks(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	installedStickerPacks, err := a.signalClient.ListInstalledStickerPacks(number)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	c.JSON(200, installedStickerPacks)
}

// @Summary Add Sticker Pack.
// @Tags Sticker Packs
// @Description In order to add a sticker pack, browse to https://signalstickers.org/ and select the sticker pack you want to add. Then, press the "Add to Signal" button. If you look at the address bar in your browser you should see an URL in this format: https://signal.art/addstickers/#pack_id=XXX&pack_key=YYY, where XXX is the pack_id and YYY is the pack_key.
// @Accept  json
// @Produce  json
// @Param number path string true "Registered Phone Number"
// @Success 204
// @Failure 400 {object} Error
// @Param data body AddStickerPackRequest true "Request"
// @Router /v1/sticker-packs/{number} [post]
func (a *Api) AddStickerPack(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}
	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	var req AddStickerPackRequest
	err = c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		return
	}

	err = a.signalClient.AddStickerPack(number, req.PackId, req.PackKey)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	c.Status(201)
}

// @Summary List Contacts
// @Tags Contacts
// @Description List all contacts for the given number.
// @Produce  json
// @Success 200 {object} []client.ListContactsResponse
// @Param number path string true "Registered Phone Number"
// @Router /v1/contacts/{number} [get]
func (a *Api) ListContacts(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}

	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	contacts, err := a.signalClient.ListContacts(number)

	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	c.JSON(200, contacts)
}

// @Summary Set Pin
// @Tags Accounts
// @Description Sets a new Signal Pin
// @Produce  json
// @Success 201
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
// @Param data body SetPinRequest true "Request"
// @Router /v1/accounts/{number}/pin [post]
func (a *Api) SetPin(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}

	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	var req SetPinRequest
	err = c.BindJSON(&req)
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - invalid request"})
		return
	}

	err = a.signalClient.SetPin(number, req.Pin)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	c.Status(201)
}

// @Summary Remove Pin
// @Tags Accounts
// @Description Removes a Signal Pin
// @Produce  json
// @Success 204
// @Failure 400 {object} Error
// @Param number path string true "Registered Phone Number"
// @Router /v1/accounts/{number}/pin [delete]
func (a *Api) RemovePin(c *gin.Context) {
	number, err := url.PathUnescape(c.Param("number"))
	if err != nil {
		c.JSON(400, Error{Msg: "Couldn't process request - malformed number"})
		return
	}

	if number == "" {
		c.JSON(400, Error{Msg: "Couldn't process request - number missing"})
		return
	}

	err = a.signalClient.RemovePin(number)
	if err != nil {
		c.JSON(400, Error{Msg: err.Error()})
		return
	}

	c.Status(204)
}
