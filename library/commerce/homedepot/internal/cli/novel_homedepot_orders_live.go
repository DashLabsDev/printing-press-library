// Copyright 2026 Greg Mushen and contributors. Licensed under Apache-2.0.
// Home Depot order-history adapter based on an observed signed-in request.
package cli

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type observedOrderSummary struct {
	OrderOrigin    string      `json:"orderOrigin"`
	OrderNumber    string      `json:"orderNumber"`
	OrderNumbers   []string    `json:"orderNumbers"`
	SalesDate      string      `json:"salesDate"`
	StoreNumber    json.Number `json:"storeNumber"`
	StoreName      string      `json:"storeName"`
	TotalAmount    float64     `json:"totalAmount"`
	RegisterNumber json.Number `json:"registerNumber"`
	TransactionID  json.Number `json:"transactionId"`
}

type observedOrderDetail struct {
	UserID            string          `json:"userId"`
	StoreNumber       json.Number     `json:"storeNumber"`
	RegisterNumber    json.Number     `json:"registerNumber"`
	TransactionID     json.Number     `json:"transactionId"`
	Error             json.RawMessage `json:"error"`
	Errors            json.RawMessage `json:"errors"`
	OrderNumber       string          `json:"orderNumber"`
	CustomerAccountID string          `json:"customerAccountId"`
	SalesDate         string          `json:"salesDate"`
	StatusDescription string          `json:"statusDescription"`
	SubTotalAmount    float64         `json:"subTotalAmount"`
	TaxTotalAmount    float64         `json:"taxTotalAmount"`
	GrandTotalAmount  float64         `json:"grandTotalAmount"`
	FulfillmentGroups []struct {
		LineItems []struct {
			Description       string  `json:"description"`
			CurrentQuantity   float64 `json:"currentQuantity"`
			CancelledQuantity float64 `json:"cancelledQuantity"`
			TotalPrice        float64 `json:"totalPrice"`
		} `json:"lineItems"`
	} `json:"fulfillmentGroups"`
}

func (o observedOrderSummary) numbers() []string {
	if len(o.OrderNumbers) > 0 {
		return o.OrderNumbers
	}
	if o.OrderNumber != "" {
		return []string{o.OrderNumber}
	}
	return nil
}

func (o observedOrderSummary) reference() string {
	if numbers := o.numbers(); o.OrderOrigin == "online" && len(numbers) > 0 {
		return numbers[0]
	}
	if o.TransactionID == "" || o.SalesDate == "" {
		return ""
	}
	return "instore:" + strings.TrimSpace(o.SalesDate[:min(10, len(o.SalesDate))]) + ":" + o.StoreNumber.String() + ":" + o.RegisterNumber.String() + ":" + o.TransactionID.String()
}

// listHomeDepotOrders uses the browser-observed orderhistory body. The
// community client omits customerAccountId from this request, which causes
// Home Depot to reject an otherwise valid signed-in session.
func listHomeDepotOrders(ctx context.Context, flags *rootFlags, start, end time.Time, maxPages int) ([]observedOrderSummary, error) {
	if end.Before(start) {
		return nil, fmt.Errorf("end date is before start date")
	}
	_, creds, err := loadHomeDepotSession()
	if err != nil {
		return nil, err
	}
	if creds.CustomerAccountID == "" {
		return nil, fmt.Errorf("Home Depot session has no customer account ID")
	}
	c, err := newHomeDepotSessionClient(flags)
	if err != nil {
		return nil, err
	}
	requestPath := fmt.Sprintf("/oms/customer/order/v1/user/%s/orderhistory", creds.UserID)
	c.NoCache = true
	headers := homeDepotOrderHeaders()
	const pageSize = 20
	orders := make([]observedOrderSummary, 0)
	seenPages := map[[32]byte]bool{}
	for page := 1; page <= maxPages; page++ {
		body := map[string]any{"orderHistoryRequest": map[string]any{
			"pageSize": pageSize, "pageNumber": page,
			"startDate": start.Format("2006-01-02"), "endDate": end.Format("2006-01-02"),
			"customerAccountId": creds.CustomerAccountID,
			"sortBy":            "salesDate", "sortOrder": "desc", "searchType": "ORDERS",
			"resultsFilter": "allOrders", "timezone": time.Now().Location().String(), "searchValue": "",
		}}
		raw, status, err := c.PostQueryWithParamsAndHeaders(ctx, requestPath, nil, body, headers)
		if err != nil {
			return nil, homeDepotRequestError("read Home Depot order history", status, err)
		}
		pageOrders, _, err := decodeHomeDepotHistory(raw, creds.UserID, creds.CustomerAccountID)
		if err != nil {
			return nil, err
		}
		// orderCount is page-specific and can count grouped purchases differently
		// from the returned summaries. An authenticated empty page is the end.
		if len(pageOrders) == 0 {
			return orders, nil
		}

		encoded, _ := json.Marshal(pageOrders)
		fingerprint := sha256.Sum256(encoded)
		if seenPages[fingerprint] {
			return nil, fmt.Errorf("order history repeated a page; results are incomplete")
		}
		seenPages[fingerprint] = true
		for _, order := range pageOrders {
			if order.reference() == "" {
				return nil, fmt.Errorf("order history returned a missing order reference")
			}
		}

		orders = append(orders, pageOrders...)

	}
	return nil, fmt.Errorf("Home Depot order history exceeded --max-scan-pages=%d; widen that limit or narrow the date range", maxPages)
}

func getHomeDepotOrder(ctx context.Context, flags *rootFlags, order observedOrderSummary) (observedOrderDetail, error) {
	_, creds, err := loadHomeDepotSession()
	if err != nil {
		return observedOrderDetail{}, err
	}
	c, err := newHomeDepotSessionClient(flags)
	if err != nil {
		return observedOrderDetail{}, err
	}
	request := map[string]any{
		"customerAccountId":     creds.CustomerAccountID,
		"groupAndSortBy":        map[string]any{"groupBy": "fulfillmentType", "filterResponse": true},
		"requestOriginatedFrom": "purchaseHistory", "timezone": time.Now().Location().String(),
	}
	if order.OrderOrigin == "online" {
		numbers := order.numbers()
		if len(numbers) == 0 {
			return observedOrderDetail{}, fmt.Errorf("online order has no number")
		}
		request["searchType"] = "online"
		request["orderNumber"] = numbers[0]
	} else {
		store, err := order.StoreNumber.Int64()
		if err != nil {
			return observedOrderDetail{}, fmt.Errorf("invalid store number")
		}
		register, err := order.RegisterNumber.Int64()
		if err != nil {
			return observedOrderDetail{}, fmt.Errorf("invalid register number")
		}
		transaction, err := order.TransactionID.Int64()
		if err != nil {
			return observedOrderDetail{}, fmt.Errorf("invalid transaction ID")
		}
		if len(order.SalesDate) < 10 {
			return observedOrderDetail{}, fmt.Errorf("invalid in-store sales date")
		}
		request["searchType"] = "instore"
		request["storeNumber"] = store
		request["registerNumber"] = register
		request["transactionId"] = transaction
		request["salesDate"] = order.SalesDate[:10]
		request["orderNumber"] = ""
	}
	requestPath := fmt.Sprintf("/oms/customer/order/v1/user/%s/orderdetails", creds.UserID)
	c.NoCache = true
	headers := homeDepotOrderHeaders()
	raw, status, err := c.PostQueryWithParamsAndHeaders(ctx, requestPath, nil, map[string]any{"orderDetailsRequest": request}, headers)
	if err != nil {
		return observedOrderDetail{}, homeDepotRequestError("read Home Depot order details", status, err)
	}
	var detail observedOrderDetail
	if err := json.Unmarshal(raw, &detail); err != nil {
		return observedOrderDetail{}, fmt.Errorf("decode Home Depot order details: %w", err)
	}
	if err := validateHomeDepotOrderDetail(raw, detail, order, creds.UserID, creds.CustomerAccountID); err != nil {
		return observedOrderDetail{}, err
	}
	return detail, nil
}

func homeDepotOrderHeaders() map[string]string {
	return map[string]string{"Accept": "application/json, text/plain, */*", "channel": "desktop", "channelId": "1", "Client": "ocm_pd_experience_customer-account-orders-purchases", "X-Client-App": "PHX-Desktop"}
}

func decodeHomeDepotHistory(raw []byte, userID, accountID string) ([]observedOrderSummary, int, error) {
	var result struct {
		UserID    string                 `json:"userId"`
		AccountID string                 `json:"customerAccountId"`
		Count     *int                   `json:"orderCount"`
		Orders    []observedOrderSummary `json:"orders"`
		Error     json.RawMessage        `json:"error"`
		Errors    json.RawMessage        `json:"errors"`
	}
	if json.Unmarshal(raw, &result) != nil || result.Count == nil || *result.Count < 0 || result.UserID != userID || result.AccountID != accountID || (len(result.Error) > 0 && string(result.Error) != "null") || (len(result.Errors) > 0 && string(result.Errors) != "null") {
		return nil, 0, authErr(fmt.Errorf("Home Depot did not return order history for the signed-in account; check 'auth status --verify'"))
	}
	if result.Orders == nil {
		result.Orders = make([]observedOrderSummary, 0)
	}
	if *result.Count > 0 && len(result.Orders) == 0 {
		return nil, 0, fmt.Errorf("order history has a positive count but no orders; results are incomplete")
	}
	return result.Orders, *result.Count, nil
}

func validateHomeDepotOrderDetail(raw []byte, detail observedOrderDetail, order observedOrderSummary, userID, accountID string) error {
	var shape map[string]json.RawMessage
	if json.Unmarshal(raw, &shape) != nil {
		return apiErr(fmt.Errorf("invalid Home Depot order details"))
	}
	failed := (len(detail.Error) > 0 && string(detail.Error) != "null") || (len(detail.Errors) > 0 && string(detail.Errors) != "null")
	if failed || detail.CustomerAccountID != accountID || detail.UserID != userID {
		return apiErr(fmt.Errorf("Home Depot did not return details for the signed-in account"))
	}
	if len(detail.SalesDate) < 10 || detail.FulfillmentGroups == nil || shape["grandTotalAmount"] == nil || string(shape["grandTotalAmount"]) == "null" {
		return apiErr(fmt.Errorf("Home Depot returned incomplete order details"))
	}
	if _, err := time.Parse("2006-01-02", detail.SalesDate[:10]); err != nil {
		return apiErr(fmt.Errorf("Home Depot returned an invalid order date"))
	}
	if order.OrderOrigin == "online" {
		if len(order.numbers()) == 0 || detail.OrderNumber != order.numbers()[0] || detail.StatusDescription == "" {
			return apiErr(fmt.Errorf("Home Depot returned details for a different or incomplete online order"))
		}
	} else if len(order.SalesDate) < 10 || detail.SalesDate[:10] != order.SalesDate[:10] || detail.StoreNumber != order.StoreNumber || detail.RegisterNumber != order.RegisterNumber || detail.TransactionID != order.TransactionID {
		return apiErr(fmt.Errorf("Home Depot returned details for a different in-store purchase"))
	}
	return nil
}
