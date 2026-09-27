package domain

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
	ErrInvalid  = errors.New("invalid input")
)

func NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	s := hex.EncodeToString(b[:])
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]
}
func ValidID(s string) bool {
	if len(s) != 36 || s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return false
	}
	_, err := hex.DecodeString(s[:8] + s[9:13] + s[14:18] + s[19:23] + s[24:])
	return err == nil
}

type User struct {
	ID                  string    `json:"id"`
	Timezone            string    `json:"timezone"`
	NotificationEnabled bool      `json:"notification_enabled"`
	SuggestionEnabled   bool      `json:"suggestion_enabled"`
	CreatedAt           time.Time `json:"created_at"`
}
type Product struct {
	ID                 string  `json:"id"`
	SKU                string  `json:"sku"`
	Name               string  `json:"name"`
	Category           string  `json:"category"`
	Brand              string  `json:"brand"`
	Unit               string  `json:"unit"`
	ReplenishableScore float64 `json:"replenishable_score"`
	Available          bool    `json:"available"`
}
type Purchase struct {
	ID          string         `json:"id"`
	UserID      string         `json:"user_id"`
	PurchasedAt time.Time      `json:"purchased_at"`
	Source      string         `json:"source"`
	TotalAmount int64          `json:"total_amount"`
	Items       []PurchaseItem `json:"items"`
}
type PurchaseItem struct {
	ProductID string `json:"product_id"`
	Quantity  int64  `json:"quantity"`
	UnitPrice int64  `json:"unit_price"`
}
type Observation struct {
	PurchasedAt time.Time
	Quantity    int64
}
type Membership struct {
	InCart       bool  `json:"in_cart"`
	InList       bool  `json:"in_list"`
	CartQuantity int64 `json:"cart_quantity"`
	// AutoAdded marks a cart line added after the user confirmed a running-low question.
	AutoAdded bool `json:"auto_added"`
}

const MaxCartQuantity = 999

type Event struct {
	ID           string          `json:"id"`
	UserID       string          `json:"user_id"`
	Type         string          `json:"event_type"`
	ProductID    *string         `json:"product_id,omitempty"`
	DecisionID   *string         `json:"decision_id,omitempty"`
	SuggestionID *string         `json:"suggestion_id,omitempty"`
	Metadata     json.RawMessage `json:"metadata"`
	CreatedAt    time.Time       `json:"created_at"`
}
type SuggestionItem struct {
	ProductID  string `json:"product_id"`
	DecisionID string `json:"decision_id"`
}
type Suggestion struct {
	ID        string           `json:"id"`
	UserID    string           `json:"user_id"`
	Kind      string           `json:"kind"`
	Message   string           `json:"message"`
	Section   string           `json:"section"`
	Status    string           `json:"status"`
	Items     []SuggestionItem `json:"items"`
	CreatedAt time.Time        `json:"created_at"`
}
