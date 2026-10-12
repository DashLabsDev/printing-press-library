package cli

import (
	"github.com/spf13/cobra"
	"strconv"
	"time"
)

func mockMoney(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }

func mockHostfullyReservations(now time.Time) []reservation {
	paid := true
	return []reservation{
		{
			UID:           "mock-hostfully-reservation-unit-4",
			PropertyUID:   "mock-hostfully-property-unit-4",
			Unit:          4,
			Status:        "BOOKED",
			CheckIn:       now.AddDate(0, 0, -1),
			CheckOut:      now.AddDate(0, 0, 2),
			Channel:       "direct",
			GuestFirst:    "Mock",
			GuestLast:     "Guest",
			Phone:         "+15550100404",
			Email:         "mock.guest@example.invalid",
			Adults:        2,
			BookingType:   "directBooking",
			Confirmation:  "MOCK-UNIT-4",
			PaymentStatus: "paid",
			Paid:          &paid,
			TotalAmount:   mockMoney(200),
			AmountPaid:    mockMoney(200),
			Balance:       mockMoney(0),
			Currency:      "USD",
			DoorPIN:       "4404",
		},
	}
}

func printMockInbox(cmd *cobra.Command, flags *rootFlags) error {
	thread := inboxThread{UID: "mock-thread"}
	message := inboxMessage{UID: "mock-message", Content: inboxContent{Text: "Mock guest message"}}
	result := map[string]any{"mock": true}
	switch cmd.Name() {
	case "show":
		result["thread"] = thread
		result["messages"] = []inboxMessage{message}
	case "message":
		result["message"] = message
	case "messages":
		result["messages"] = []inboxMessage{message}
	default:
		result["threads"] = []inboxThread{thread}
	}
	return printJSONFiltered(cmd.OutOrStdout(), result, flags)
}
