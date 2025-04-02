package handlers

func getClientCount() int {
	clientM.RLock()
	defer clientM.RUnlock()
	return len(clients)
}
