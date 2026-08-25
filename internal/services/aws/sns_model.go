package aws

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SNSTopic contains the topic fields rendered by the browser. Attributes come
// from GetTopicAttributes, which is a per-topic call, so a topic whose
// attribute lookup was denied still appears with AttributesKnown false.
type SNSTopic struct {
	ARN                       string
	Name                      string
	DisplayName               string
	Region                    string
	KMSMasterKeyID            string
	DeliveryPolicy            string
	EffectiveDeliveryPolicy   string
	SubscriptionsConfirmed    int
	SubscriptionsPending      int
	SubscriptionsDeleted      int
	FIFO                      bool
	ContentBasedDeduplication bool
	AttributesKnown           bool
}

// IsFIFO reports whether the topic is a FIFO topic. SNS marks these with a
// .fifo ARN suffix as well as the FifoTopic attribute, so the name is used as
// a fallback when attributes could not be read.
func (t SNSTopic) IsFIFO() bool {
	return t.FIFO || strings.HasSuffix(t.Name, ".fifo")
}

// KindLabel renders the topic type for the list column.
func (t SNSTopic) KindLabel() string {
	if t.IsFIFO() {
		return "FIFO"
	}
	return "standard"
}

// SubscriptionSummary renders confirmed/pending counts, or a dash when the
// attribute lookup failed and the counts are unknown rather than zero.
func (t SNSTopic) SubscriptionSummary() string {
	if !t.AttributesKnown {
		return "-"
	}
	if t.SubscriptionsPending > 0 {
		return fmt.Sprintf("%d (+%d pending)", t.SubscriptionsConfirmed, t.SubscriptionsPending)
	}
	return fmt.Sprintf("%d", t.SubscriptionsConfirmed)
}

// FilterText returns searchable topic metadata.
func (t SNSTopic) FilterText() string {
	return strings.ToLower(strings.Join([]string{
		t.Name, t.ARN, t.DisplayName, t.KindLabel(), t.Region, t.KMSMasterKeyID,
	}, " "))
}

// DisplayTitle returns the palette-facing label.
func (t SNSTopic) DisplayTitle() string { return t.Name }

// SNSSubscription describes one subscription attached to a topic.
type SNSSubscription struct {
	ARN                string
	Protocol           string
	Endpoint           string
	Owner              string
	TopicARN           string
	RawMessageDelivery bool
	RedrivePolicy      string
	FilterPolicy       string
	AttributesKnown    bool
}

// Confirmed reports whether the subscription has completed confirmation. SNS
// reports unconfirmed subscriptions with a sentinel ARN rather than a status
// field.
func (s SNSSubscription) Confirmed() bool {
	switch s.ARN {
	case "", "PendingConfirmation", "Deleted":
		return false
	}
	return strings.HasPrefix(s.ARN, "arn:")
}

// Status renders the subscription lifecycle state for display.
func (s SNSSubscription) Status() string {
	switch s.ARN {
	case "PendingConfirmation":
		return "pending"
	case "Deleted":
		return "deleted"
	case "":
		return "-"
	}
	if s.Confirmed() {
		return "confirmed"
	}
	return s.ARN
}

// HasRedrive reports whether the subscription has a dead-letter queue attached.
func (s SNSSubscription) HasRedrive() bool { return strings.TrimSpace(s.RedrivePolicy) != "" }

// DeadLetterTargetARN returns the queue ARN from the subscription redrive
// policy, or an empty string when the policy is absent or malformed.
func (s SNSSubscription) DeadLetterTargetARN() string {
	var policy struct {
		DeadLetterTargetARN string `json:"deadLetterTargetArn"`
	}
	if json.Unmarshal([]byte(s.RedrivePolicy), &policy) != nil {
		return ""
	}
	return policy.DeadLetterTargetARN
}

// FilterText returns searchable subscription metadata.
func (s SNSSubscription) FilterText() string {
	return strings.ToLower(strings.Join([]string{
		s.Protocol, s.Endpoint, s.Owner, s.Status(), s.ARN,
	}, " "))
}
