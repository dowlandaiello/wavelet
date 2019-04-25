package wctl

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/perlin-network/noise/edwards25519"
	"github.com/valyala/fasthttp"
	"net/http"
	"time"
)

type Config struct {
	APIHost    string
	APIPort    uint16
	PrivateKey edwards25519.PrivateKey
	UseHTTPS   bool
}

type Client struct {
	Config

	edwards25519.PrivateKey
	edwards25519.PublicKey

	SessionToken string
}

func NewClient(config Config) (*Client, error) {
	return &Client{Config: config, PrivateKey: config.PrivateKey, PublicKey: config.PrivateKey.Public()}, nil
}

// Request will make a request to a given path, with a given body and return result in out.
func (c *Client) RequestJSON(path string, method string, body MarshalableJSON, out UnmarshalableJSON) error {
	resBody, err := c.Request(path, method, body)
	if err != nil {
		return err
	}

	if out == nil {
		return nil
	}

	return out.UnmarshalJSON(resBody)
}

func (c *Client) Request(path string, method string, body MarshalableJSON) ([]byte, error) {
	protocol := "http"
	if c.Config.UseHTTPS {
		protocol = "https"
	}

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	req.URI().Update(fmt.Sprintf("%s://%s:%d%s", protocol, c.Config.APIHost, c.Config.APIPort, path))
	req.Header.SetMethod(method)
	req.Header.SetContentType("application/json")
	req.Header.Add(HeaderSessionToken, c.SessionToken)

	if body != nil {
		raw, err := body.MarshalJSON()
		if err != nil {
			return nil, err
		}

		req.SetBody(raw)
	}

	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(res)

	if err := fasthttp.Do(req, res); err != nil {
		return nil, err
	}

	return res.Body(), nil
}

// EstablishWS will create a websocket connection.
func (c *Client) EstablishWS(path string) (*websocket.Conn, error) {
	prot := "ws"
	if c.Config.UseHTTPS {
		prot = "wss"
	}

	url := fmt.Sprintf("%s://%s:%d%s", prot, c.Config.APIHost, c.Config.APIPort, path)

	header := make(http.Header)
	header.Add(HeaderSessionToken, c.SessionToken)

	dialer := &websocket.Dialer{}
	conn, _, err := dialer.Dial(url, header)
	return conn, err
}

// Init instantiates a new session with the Wavelet nodes HTTP API.
func (c *Client) Init() error {
	var res SessionInitResponse

	millis := time.Now().UnixNano() * 1000
	message := []byte(fmt.Sprintf("%s%d", SessionInitMessage, millis))

	signature := edwards25519.Sign(c.PrivateKey, message)

	req := SessionInitRequest{
		PublicKey:  hex.EncodeToString(c.PublicKey[:]),
		TimeMillis: uint64(millis),
		Signature:  hex.EncodeToString(signature[:]),
	}

	if err := c.RequestJSON(RouteSessionInit, ReqPost, &req, &res); err != nil {
		return err
	}

	c.SessionToken = res.Token

	return nil
}

func (c *Client) PollLoggerSink(stop <-chan struct{}, sinkRoute string) (<-chan []byte, error) {
	path := fmt.Sprintf("%s?token=%s", sinkRoute, c.SessionToken)

	if stop == nil {
		stop = make(chan struct{})
	}

	ws, err := c.EstablishWS(path)
	if err != nil {
		return nil, err
	}

	evChan := make(chan []byte)

	go func() {
		defer close(evChan)

		for {
			_, message, err := ws.ReadMessage()
			if err != nil {
				return
			}

			select {
			case <-stop:
				return
			case evChan <- message:
			}
		}
	}()

	return evChan, nil
}

func (c *Client) PollAccounts(stop <-chan struct{}, accountID *string) (<-chan []byte, error) {
	path := fmt.Sprintf("%s?token=%s", RouteWSAccounts, c.SessionToken)
	if accountID != nil {
		path = fmt.Sprintf("%s&id=%s", path, *accountID)
	}

	if stop == nil {
		stop = make(chan struct{})
	}

	ws, err := c.EstablishWS(path)
	if err != nil {
		return nil, err
	}

	evChan := make(chan []byte)

	go func() {
		defer close(evChan)

		for {
			_, message, err := ws.ReadMessage()
			if err != nil {
				return
			}

			select {
			case <-stop:
				return
			case evChan <- message:
			}
		}
	}()

	return evChan, nil
}

func (c *Client) PollContracts(stop <-chan struct{}, contractID *string) (<-chan []byte, error) {
	path := fmt.Sprintf("%s?token=%s", RouteWSContracts, c.SessionToken)
	if contractID != nil {
		path = fmt.Sprintf("%sid=%s&", path, *contractID)
	}

	if stop == nil {
		stop = make(chan struct{})
	}

	ws, err := c.EstablishWS(path)
	if err != nil {
		return nil, err
	}

	evChan := make(chan []byte)

	go func() {
		defer close(evChan)

		for {
			_, message, err := ws.ReadMessage()
			if err != nil {
				return
			}

			select {
			case <-stop:
				return
			case evChan <- message:
			}
		}
	}()

	return evChan, nil
}

func (c *Client) PollTransactions(stop <-chan struct{}, txID *string, senderID *string, creatorID *string) (<-chan []byte, error) {
	path := fmt.Sprintf("%s?token=%s", RouteWSTransactions, c.SessionToken)
	if txID != nil {
		path = fmt.Sprintf("%stx_id=%s&", path, *txID)
	}
	if senderID != nil {
		path = fmt.Sprintf("%ssender=%s&", path, *senderID)
	}
	if creatorID != nil {
		path = fmt.Sprintf("%screator=%s&", path, *creatorID)
	}

	if stop == nil {
		stop = make(chan struct{})
	}

	ws, err := c.EstablishWS(path)
	if err != nil {
		return nil, err
	}

	evChan := make(chan []byte)

	go func() {
		defer close(evChan)

		for {
			_, message, err := ws.ReadMessage()
			if err != nil {
				return
			}

			select {
			case <-stop:
				return
			case evChan <- message:
			}
		}
	}()

	return evChan, nil
}

func (c *Client) GetLedgerStatus(senderID *string, creatorID *string, offset *uint64, limit *uint64) (LedgerStatusResponse, error) {
	path := fmt.Sprintf("%s?", RouteLedger)
	if senderID != nil {
		path = fmt.Sprintf("%ssender=%s&", path, *senderID)
	}
	if creatorID != nil {
		path = fmt.Sprintf("%screator=%s&", path, *creatorID)
	}
	if offset != nil {
		path = fmt.Sprintf("%soffset=%d&", path, *offset)
	}
	if limit != nil {
		path = fmt.Sprintf("%slimit=%d&", path, *limit)
	}

	var res LedgerStatusResponse
	err := c.RequestJSON(path, ReqGet, nil, &res)
	return res, err
}

func (c *Client) GetAccount(accountID string) (Account, error) {
	path := fmt.Sprintf("%s/%s", RouteAccount, accountID)

	var res Account
	err := c.RequestJSON(path, ReqGet, nil, &res)
	return res, err
}

func (c *Client) GetContractCode(contractID string) (string, error) {
	path := fmt.Sprintf("%s/%s", RouteContract, contractID)

	res, err := c.Request(path, ReqGet, nil)
	return string(res), err
}

func (c *Client) GetContractPages(contractID string, index *uint64) (string, error) {
	path := fmt.Sprintf("%s/%s/page", RouteContract, contractID)
	if index != nil {
		path = fmt.Sprintf("%s/%d", path, *index)
	}

	res, err := c.Request(path, ReqGet, nil)
	return base64.StdEncoding.EncodeToString(res), err
}

func (c *Client) ListTransactions(senderID *string, creatorID *string, offset *uint64, limit *uint64) ([]Transaction, error) {
	path := fmt.Sprintf("%s?", RouteTxList)
	if senderID != nil {
		path = fmt.Sprintf("%ssender=%s&", path, *senderID)
	}
	if creatorID != nil {
		path = fmt.Sprintf("%screator=%s&", path, *creatorID)
	}
	if offset != nil {
		path = fmt.Sprintf("%soffset=%d&", path, *offset)
	}
	if limit != nil {
		path = fmt.Sprintf("%slimit=%d&", path, *limit)
	}

	var res TransactionList

	err := c.RequestJSON(path, ReqGet, nil, &res)
	return res, err

}

func (c *Client) GetTransaction(txID string) (Transaction, error) {
	path := fmt.Sprintf("%s/%s", RouteTxList, txID)

	var res Transaction
	err := c.RequestJSON(path, ReqGet, nil, &res)
	return res, err
}

func (c *Client) SendTransaction(tag byte, payload []byte) (SendTransactionResponse, error) {
	var res SendTransactionResponse

	var nonce [8]byte // TODO(kenta): nonce

	signature := edwards25519.Sign(c.PrivateKey, append(nonce[:], append([]byte{tag}, payload...)...))

	req := SendTransactionRequest{
		Sender:    hex.EncodeToString(c.PublicKey[:]),
		Tag:       tag,
		Payload:   hex.EncodeToString(payload),
		Signature: hex.EncodeToString(signature[:]),
	}

	err := c.RequestJSON(RouteTxSend, ReqPost, &req, &res)

	return res, err
}
