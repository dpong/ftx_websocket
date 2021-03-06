package websocket

import (
	"errors"
	"log"
)

func (c *RecConnClientWS) Decoding(message []byte, logger *log.Logger) (res map[string]interface{}, err error) {
	if message == nil {
		err = errors.New("the incoming message is nil")
		logger.Println(err)
		return nil, err
	}
	err = json.Unmarshal(message, &res)
	if err != nil {
		logger.Println(err)
		return nil, err
	}
	return res, nil
}
