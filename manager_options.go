package gorabbit

import "github.com/Netflix/go-env"

// ManagerOptions holds all necessary properties to launch a successful connection with an MQTTManager.
type ManagerOptions struct {
	// Host is the RabbitMQ server host name.
	Host string

	// Port is the RabbitMQ server port number.
	Port uint

	// Username is the RabbitMQ server allowed username.
	Username string

	// Password is the RabbitMQ server allowed password.
	Password string

	// Vhost is used for CloudAMQP connections to set the specific vhost.
	Vhost string

	// UseTLS defines whether we use amqp or amqps protocol.
	UseTLS bool

	// Mode will specify whether logs are enabled or not.
	Mode string
}

// DefaultManagerOptions will return a ManagerOptions with default values.
func DefaultManagerOptions() *ManagerOptions {
	return &ManagerOptions{
		Host:     defaultHost,
		Port:     defaultPort,
		Username: defaultUsername,
		Password: defaultPassword,
		Vhost:    defaultVhost,
		UseTLS:   defaultUseTLS,
		Mode:     defaultMode,
	}
}

// NewManagerOptions is the exported builder for a ManagerOptions and will offer setter methods for an easy construction.
// Any non-assigned field will be set to default through DefaultManagerOptions.
func NewManagerOptions() *ManagerOptions {
	return DefaultManagerOptions()
}

// NewManagerOptionsFromEnv will generate a ManagerOptions from environment variables. Empty values will be taken as default
// through the DefaultManagerOptions.
func NewManagerOptionsFromEnv() *ManagerOptions {
	defaultOpts := DefaultManagerOptions()

	fromEnv := new(RabbitMQEnvs)

	_, err := env.UnmarshalFromEnviron(fromEnv)
	if err != nil {
		return defaultOpts
	}

	if fromEnv.Host != "" {
		defaultOpts.Host = fromEnv.Host
	}

	if fromEnv.Port > 0 {
		defaultOpts.Port = fromEnv.Port
	}

	if fromEnv.Username != "" {
		defaultOpts.Username = fromEnv.Username
	}

	if fromEnv.Password != "" {
		defaultOpts.Password = fromEnv.Password
	}

	if fromEnv.Vhost != "" {
		defaultOpts.Vhost = fromEnv.Vhost
	}

	defaultOpts.UseTLS = fromEnv.UseTLS

	return defaultOpts
}

// SetHost will assign the host.
func (m *ManagerOptions) SetHost(host string) *ManagerOptions {
	m.Host = host

	return m
}

// SetPort will assign the port.
func (m *ManagerOptions) SetPort(port uint) *ManagerOptions {
	m.Port = port

	return m
}

// SetCredentials will assign the username and password.
func (m *ManagerOptions) SetCredentials(username, password string) *ManagerOptions {
	m.Username = username
	m.Password = password

	return m
}

// SetVhost will assign the Vhost.
func (m *ManagerOptions) SetVhost(vhost string) *ManagerOptions {
	m.Vhost = vhost

	return m
}

// SetUseTLS will assign the UseTLS status.
func (m *ManagerOptions) SetUseTLS(use bool) *ManagerOptions {
	m.UseTLS = use

	return m
}

// SetMode will assign the mode if valid.
func (m *ManagerOptions) SetMode(mode string) *ManagerOptions {
	if isValidMode(mode) {
		m.Mode = mode
	}

	return m
}
