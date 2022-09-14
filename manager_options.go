package gorabbit

// ManagerOptions holds all necessary properties to launch a successful connection with an MQTTManager.
type ManagerOptions struct {
	// host is the RabbitMQ server host name.
	host string

	// port is the RabbitMQ server port number.
	port uint

	// username is the RabbitMQ server allowed username.
	username string

	// password is the RabbitMQ server allowed password.
	password string

	// mode will specify whether logs are enabled or not.
	mode string
}

// DefaultManagerOptions will return a ManagerOptions with default values.
func DefaultManagerOptions() *ManagerOptions {
	return &ManagerOptions{
		host:     defaultHost,
		port:     defaultPort,
		username: defaultUsername,
		password: defaultPassword,
		mode:     defaultMode,
	}
}

// NewManagerOptions is the exported builder for a ManagerOptions and will offer setter methods for an easy construction.
// Any non-assigned field will be set to default through DefaultManagerOptions.
func NewManagerOptions() *ManagerOptions {
	return DefaultManagerOptions()
}

// SetHost will assign the host.
func (m *ManagerOptions) SetHost(host string) *ManagerOptions {
	m.host = host

	return m
}

// SetPort will assign the port.
func (m *ManagerOptions) SetPort(port uint) *ManagerOptions {
	m.port = port

	return m
}

// SetCredentials will assign the username and password.
func (m *ManagerOptions) SetCredentials(username, password string) *ManagerOptions {
	m.username = username
	m.password = password

	return m
}

// SetMode will assign the mode if valid.
func (m *ManagerOptions) SetMode(mode string) *ManagerOptions {
	if isValidMode(mode) {
		m.mode = mode
	}

	return m
}
