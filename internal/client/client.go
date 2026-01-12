package client

import (
	"sync"
)

type HeadpatMessage struct {
	Count     int    `json:"count"`
	Total     int    `json:"total"`
	Timestamp string `json:"timestamp"`
}

type ClientManager struct {
	clientsMap   map[chan HeadpatMessage]bool
	clientsMutex sync.RWMutex
}

func NewClientManager() ClientManager {
	return ClientManager{clientsMap: make(map[chan HeadpatMessage]bool)}
}

func (cm *ClientManager) NewClient() chan HeadpatMessage {
	cm.clientsMutex.Lock()
	defer cm.clientsMutex.Unlock()

	client := make(chan HeadpatMessage)
	cm.clientsMap[client] = true
	return client
}

func (cm *ClientManager) CloseClient(client chan HeadpatMessage) {
	cm.clientsMutex.Lock()
	defer cm.clientsMutex.Unlock()

	delete(cm.clientsMap, client)
	close(client)
}

func (cm *ClientManager) SendAll(msg HeadpatMessage) {
	cm.clientsMutex.RLock()
	for client := range cm.clientsMap {
		client <- msg
	}
	cm.clientsMutex.RUnlock()
}
