package gorabbit

type ConnectionStatus string

const (
	ConnUp   ConnectionStatus = "connUp"
	ConnDown ConnectionStatus = "connDown"
	ChanUp   ConnectionStatus = "canUp"
	ChanDown ConnectionStatus = "chanDown"
)
