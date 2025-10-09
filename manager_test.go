package gorabbit_test

import (
	"context"
	"log"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/KardinalAI/gorabbit"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/rabbitmq"
)

type RabbitMQContainer struct {
	*rabbitmq.RabbitMQContainer
	ContainerHost string
	ContainerPort uint
	Username      string
	Password      string
}

func CreateRabbitMQContainer(ctx context.Context) (*RabbitMQContainer, error) {
	rContainer, err := rabbitmq.RunContainer(ctx,
		testcontainers.WithImage("rabbitmq:3.12.11-management-alpine"),
		rabbitmq.WithAdminUsername("guest"),
		rabbitmq.WithAdminPassword("guest"),
	)

	if err != nil {
		return nil, err
	}

	ep, err := rContainer.AmqpURL(ctx)
	if err != nil {
		return nil, err
	}

	uri, err := amqp.ParseURI(ep)
	if err != nil {
		return nil, err
	}

	rabbitMQContainer := &RabbitMQContainer{
		RabbitMQContainer: rContainer,
		ContainerHost:     uri.Host,
		ContainerPort:     uint(uri.Port),
		Username:          uri.Username,
		Password:          uri.Password,
	}

	return rabbitMQContainer, nil
}

type ManagerTestSuite struct {
	suite.Suite
	rabbitMQContainer *RabbitMQContainer
	ctx               context.Context
}

func (suite *ManagerTestSuite) SetupSuite() {
	suite.ctx = context.Background()

	rContainer, err := CreateRabbitMQContainer(suite.ctx)
	if err != nil {
		log.Fatal(err)
	}

	suite.rabbitMQContainer = rContainer
}

func (suite *ManagerTestSuite) TearDownSuite() {
	if err := suite.rabbitMQContainer.Terminate(suite.ctx); err != nil {
		log.Fatalf("error terminating RabbitMQ container: %s", err)
	}
}

func (suite *ManagerTestSuite) TestNewManager() {
	t := suite.T()

	t.Run("Instantiating new manager with correct parameters", func(t *testing.T) {
		managerOpts := gorabbit.NewManagerOptions().
			SetHost(suite.rabbitMQContainer.ContainerHost).
			SetPort(suite.rabbitMQContainer.ContainerPort).
			SetCredentials(suite.rabbitMQContainer.Username, suite.rabbitMQContainer.Password)

		manager, err := gorabbit.NewManager(managerOpts)

		require.NoError(t, err)
		assert.NotNil(t, manager)

		assert.NotEmpty(t, manager.GetHost())
		assert.NotZero(t, manager.GetPort())
		assert.NotEmpty(t, manager.GetUsername())

		require.NoError(t, manager.Disconnect())
	})

	t.Run("Instantiating new manager with incorrect credentials", func(t *testing.T) {
		managerOpts := gorabbit.NewManagerOptions().
			SetHost(suite.rabbitMQContainer.ContainerHost).
			SetPort(suite.rabbitMQContainer.ContainerPort).
			SetCredentials("bad", "password")

		manager, err := gorabbit.NewManager(managerOpts)

		require.Error(t, err)
		assert.Equal(t, "Exception (403) Reason: \"username or password not allowed\"", err.Error())
		assert.NotNil(t, manager)

		// Running any operation at that point should fail, except for disconnecting
		require.NoError(t, manager.Disconnect())

		err = manager.CreateQueue(gorabbit.QueueConfig{})

		require.Error(t, err)
		assert.Equal(t, "connection is closed", err.Error())
	})

	t.Run("Instantiating new manager with incorrect host", func(t *testing.T) {
		managerOpts := gorabbit.NewManagerOptions().
			SetHost("incorrect_host").
			SetPort(suite.rabbitMQContainer.ContainerPort).
			SetCredentials(suite.rabbitMQContainer.Username, suite.rabbitMQContainer.Password)

		manager, err := gorabbit.NewManager(managerOpts)

		require.Error(t, err)

		hasDialPrefix := strings.HasPrefix(err.Error(), "dial tcp: lookup incorrect_host on")
		assert.True(t, hasDialPrefix)

		hasDialSuffix := strings.HasSuffix(err.Error(), "server misbehaving")
		assert.True(t, hasDialSuffix)

		assert.NotNil(t, manager)

		// Running any operation at that point should fail, except for disconnecting
		require.NoError(t, manager.Disconnect())

		err = manager.CreateQueue(gorabbit.QueueConfig{})

		require.Error(t, err)
		assert.Equal(t, "connection is closed", err.Error())
	})

	t.Run("Instantiating new manager with incorrect port", func(t *testing.T) {
		managerOpts := gorabbit.NewManagerOptions().
			SetHost(suite.rabbitMQContainer.ContainerHost).
			SetPort(uint(123)).
			SetCredentials(suite.rabbitMQContainer.Username, suite.rabbitMQContainer.Password)

		manager, err := gorabbit.NewManager(managerOpts)

		require.Error(t, err)

		hasDialPrefix := strings.HasPrefix(err.Error(), "dial tcp")
		assert.True(t, hasDialPrefix)

		hasDialSuffix := strings.HasSuffix(err.Error(), "connection refused")
		assert.True(t, hasDialSuffix)

		assert.NotNil(t, manager)

		// Running any operation at that point should fail, except for disconnecting
		require.NoError(t, manager.Disconnect())

		err = manager.CreateQueue(gorabbit.QueueConfig{})

		require.Error(t, err)
		assert.Equal(t, "connection is closed", err.Error())
	})
}

func (suite *ManagerTestSuite) TestNewFromEnv() {
	t := suite.T()

	t.Setenv("RABBITMQ_HOST", suite.rabbitMQContainer.ContainerHost)
	t.Setenv("RABBITMQ_PORT", strconv.Itoa(int(suite.rabbitMQContainer.ContainerPort)))
	t.Setenv("RABBITMQ_USERNAME", suite.rabbitMQContainer.Username)
	t.Setenv("RABBITMQ_PASSWORD", suite.rabbitMQContainer.Password)

	manager, err := gorabbit.NewManagerFromEnv()

	require.NoError(t, err)
	assert.NotNil(t, manager)

	require.NoError(t, manager.Disconnect())

	t.Run("Instantiating a manager with disabled flag", func(t *testing.T) {
		t.Setenv("GORABBIT_DISABLED", "true")

		manager, err = gorabbit.NewManagerFromEnv()

		require.NoError(t, err)
		assert.NotNil(t, manager)

		assert.True(t, manager.IsDisabled())
	})
}

func (suite *ManagerTestSuite) TestCreateQueue() {
	t := suite.T()

	t.Setenv("RABBITMQ_HOST", suite.rabbitMQContainer.ContainerHost)
	t.Setenv("RABBITMQ_PORT", strconv.Itoa(int(suite.rabbitMQContainer.ContainerPort)))
	t.Setenv("RABBITMQ_USERNAME", suite.rabbitMQContainer.Username)
	t.Setenv("RABBITMQ_PASSWORD", suite.rabbitMQContainer.Password)

	manager, err := gorabbit.NewManagerFromEnv()

	require.NoError(t, err)
	assert.NotNil(t, manager)

	t.Run("Creating queue with valid parameters", func(t *testing.T) {
		queueConfig := gorabbit.QueueConfig{
			Name:      "test_queue",
			Durable:   true,
			Exclusive: false,
		}

		err = manager.CreateQueue(queueConfig)

		require.NoError(t, err)

		err = manager.DeleteQueue(queueConfig.Name)

		require.NoError(t, err)
	})

	t.Run("Creating queue with empty name should work", func(t *testing.T) {
		queueConfig := gorabbit.QueueConfig{}

		err = manager.CreateQueue(queueConfig)

		require.NoError(t, err)

		err = manager.DeleteQueue(queueConfig.Name)

		require.NoError(t, err)
	})

	t.Run("Creating queue with bindings but non-existent exchange", func(t *testing.T) {
		queueConfig := gorabbit.QueueConfig{
			Name:      "test_queue",
			Durable:   true,
			Exclusive: false,
			Bindings: []gorabbit.BindingConfig{
				{
					RoutingKey: "routing_key",
					Exchange:   "test_exchange",
				},
			},
		}

		err = manager.CreateQueue(queueConfig)

		require.Error(t, err)
		assert.Equal(t, "Exception (404) Reason: \"NOT_FOUND - no exchange 'test_exchange' in vhost '/'\"", err.Error())
	})

	t.Run("Creating queue with bindings and existing exchange", func(t *testing.T) {
		exchangeConfig := gorabbit.ExchangeConfig{
			Name: "test_exchange_with_bindings",
			Type: "topic",
		}

		err = manager.CreateExchange(exchangeConfig)

		require.NoError(t, err)

		queueConfig := gorabbit.QueueConfig{
			Name:      "test_queue",
			Durable:   true,
			Exclusive: false,
			Bindings: []gorabbit.BindingConfig{
				{
					RoutingKey: "routing_key",
					Exchange:   "test_exchange_with_bindings",
				},
			},
		}

		err = manager.CreateQueue(queueConfig)

		require.NoError(t, err)

		err = manager.DeleteExchange(exchangeConfig.Name)

		require.NoError(t, err)

		err = manager.DeleteQueue(queueConfig.Name)

		require.NoError(t, err)
	})

	require.NoError(t, manager.Disconnect())
}

func (suite *ManagerTestSuite) TestCreateExchange() {
	t := suite.T()

	t.Setenv("RABBITMQ_HOST", suite.rabbitMQContainer.ContainerHost)
	t.Setenv("RABBITMQ_PORT", strconv.Itoa(int(suite.rabbitMQContainer.ContainerPort)))
	t.Setenv("RABBITMQ_USERNAME", suite.rabbitMQContainer.Username)
	t.Setenv("RABBITMQ_PASSWORD", suite.rabbitMQContainer.Password)

	manager, err := gorabbit.NewManagerFromEnv()

	require.NoError(t, err)
	assert.NotNil(t, manager)

	t.Run("Creating exchange with valid parameters", func(t *testing.T) {
		exchangeConfig := gorabbit.ExchangeConfig{
			Name: "test_exchange",
			Type: gorabbit.ExchangeTypeTopic,
		}

		err = manager.CreateExchange(exchangeConfig)

		require.NoError(t, err)

		err = manager.DeleteExchange(exchangeConfig.Name)

		require.NoError(t, err)
	})

	t.Run("Creating exchange with empty parameters", func(t *testing.T) {
		exchangeConfig := gorabbit.ExchangeConfig{}

		err = manager.CreateExchange(exchangeConfig)

		require.Error(t, err)
		assert.Equal(t, "Exception (503) Reason: \"COMMAND_INVALID - invalid exchange type ''\"", err.Error())

		// By now the manager's connection should be closed
		err = manager.CreateExchange(exchangeConfig)

		require.Error(t, err)
		assert.Equal(t, "connection is closed", err.Error())

		manager, err = gorabbit.NewManagerFromEnv()

		require.NoError(t, err)
		assert.NotNil(t, manager)
	})

	t.Run("Creating exchange with empty name", func(t *testing.T) {
		exchangeConfig := gorabbit.ExchangeConfig{
			Name: "",
			Type: gorabbit.ExchangeTypeTopic,
		}

		err = manager.CreateExchange(exchangeConfig)

		require.Error(t, err)
		assert.Equal(t, "Exception (403) Reason: \"ACCESS_REFUSED - operation not permitted on the default exchange\"", err.Error())

		manager, err = gorabbit.NewManagerFromEnv()

		require.NoError(t, err)
		assert.NotNil(t, manager)
	})

	t.Run("Creating all type of exchanges", func(t *testing.T) {
		topicExchange := gorabbit.ExchangeConfig{
			Name: "topic_exchange",
			Type: gorabbit.ExchangeTypeTopic,
		}

		err = manager.CreateExchange(topicExchange)

		require.NoError(t, err)

		directExchange := gorabbit.ExchangeConfig{
			Name: "direct_exchange",
			Type: gorabbit.ExchangeTypeDirect,
		}

		err = manager.CreateExchange(directExchange)

		require.NoError(t, err)

		fanoutExchange := gorabbit.ExchangeConfig{
			Name: "fanout_exchange",
			Type: gorabbit.ExchangeTypeFanout,
		}

		err = manager.CreateExchange(fanoutExchange)

		require.NoError(t, err)

		headersExchange := gorabbit.ExchangeConfig{
			Name: "headers_exchange",
			Type: gorabbit.ExchangeTypeHeaders,
		}

		err = manager.CreateExchange(headersExchange)

		require.NoError(t, err)

		require.NoError(t, manager.DeleteExchange(topicExchange.Name))
		require.NoError(t, manager.DeleteExchange(directExchange.Name))
		require.NoError(t, manager.DeleteExchange(fanoutExchange.Name))
		require.NoError(t, manager.DeleteExchange(headersExchange.Name))
	})

	require.NoError(t, manager.Disconnect())
}

func (suite *ManagerTestSuite) TestBindExchangeToQueueViaRoutingKey() {
	t := suite.T()

	t.Setenv("RABBITMQ_HOST", suite.rabbitMQContainer.ContainerHost)
	t.Setenv("RABBITMQ_PORT", strconv.Itoa(int(suite.rabbitMQContainer.ContainerPort)))
	t.Setenv("RABBITMQ_USERNAME", suite.rabbitMQContainer.Username)
	t.Setenv("RABBITMQ_PASSWORD", suite.rabbitMQContainer.Password)

	manager, err := gorabbit.NewManagerFromEnv()

	require.NoError(t, err)
	assert.NotNil(t, manager)

	queueConfig := gorabbit.QueueConfig{
		Name:      "test_queue",
		Durable:   true,
		Exclusive: false,
	}

	err = manager.CreateQueue(queueConfig)

	require.NoError(t, err)

	exchangeConfig := gorabbit.ExchangeConfig{
		Name: "test_exchange",
		Type: "topic",
	}

	err = manager.CreateExchange(exchangeConfig)

	require.NoError(t, err)

	t.Run("Binding existing exchange to existing queue via routing key", func(t *testing.T) {
		err = manager.BindExchangeToQueueViaRoutingKey(exchangeConfig.Name, queueConfig.Name, "routing_key")

		require.NoError(t, err)
	})

	t.Run("Binding non-existing exchange to existing queue via routing key", func(t *testing.T) {
		err = manager.BindExchangeToQueueViaRoutingKey("non_existing_exchange", queueConfig.Name, "routing_key")

		require.Error(t, err)
		assert.Equal(t, "Exception (404) Reason: \"NOT_FOUND - no exchange 'non_existing_exchange' in vhost '/'\"", err.Error())
	})

	t.Run("Binding existing exchange to non-existing queue via routing key", func(t *testing.T) {
		err = manager.BindExchangeToQueueViaRoutingKey(exchangeConfig.Name, "non_existing_queue", "routing_key")

		require.Error(t, err)
		assert.Equal(t, "Exception (404) Reason: \"NOT_FOUND - no queue 'non_existing_queue' in vhost '/'\"", err.Error())
	})

	require.NoError(t, manager.DeleteQueue(exchangeConfig.Name))
	require.NoError(t, manager.DeleteQueue(queueConfig.Name))

	require.NoError(t, manager.Disconnect())
}

func (suite *ManagerTestSuite) TestUnbindExchangeFromQueueViaRoutingKey() {
	t := suite.T()

	t.Setenv("RABBITMQ_HOST", suite.rabbitMQContainer.ContainerHost)
	t.Setenv("RABBITMQ_PORT", strconv.Itoa(int(suite.rabbitMQContainer.ContainerPort)))
	t.Setenv("RABBITMQ_USERNAME", suite.rabbitMQContainer.Username)
	t.Setenv("RABBITMQ_PASSWORD", suite.rabbitMQContainer.Password)

	manager, err := gorabbit.NewManagerFromEnv()

	require.NoError(t, err)
	assert.NotNil(t, manager)

	queueConfig := gorabbit.QueueConfig{
		Name:      "test_queue",
		Durable:   true,
		Exclusive: false,
	}

	err = manager.CreateQueue(queueConfig)

	require.NoError(t, err)

	exchangeConfig := gorabbit.ExchangeConfig{
		Name: "test_exchange",
		Type: "topic",
	}

	err = manager.CreateExchange(exchangeConfig)

	require.NoError(t, err)

	err = manager.BindExchangeToQueueViaRoutingKey(exchangeConfig.Name, queueConfig.Name, "routing_key")

	require.NoError(t, err)

	t.Run("Unbinding existing exchange from existing queue via routing key", func(t *testing.T) {
		err = manager.UnbindExchangeFromQueueViaRoutingKey(exchangeConfig.Name, queueConfig.Name, "routing_key")

		require.NoError(t, err)
	})

	t.Run("Unbinding again existing exchange from existing queue via routing key", func(t *testing.T) {
		err = manager.UnbindExchangeFromQueueViaRoutingKey(exchangeConfig.Name, queueConfig.Name, "routing_key")

		require.NoError(t, err)
	})

	t.Run("Unbinding existing exchange from existing queue via unknown routing key", func(t *testing.T) {
		err = manager.UnbindExchangeFromQueueViaRoutingKey(exchangeConfig.Name, queueConfig.Name, "unk_routing_key")

		require.NoError(t, err)
	})

	t.Run("Unbinding non-existing exchange from existing queue via routing key", func(t *testing.T) {
		err = manager.UnbindExchangeFromQueueViaRoutingKey("non_existing_exchange", queueConfig.Name, "routing_key")

		require.NoError(t, err)
	})

	t.Run("Unbinding existing exchange from non-existing queue via routing key", func(t *testing.T) {
		err = manager.UnbindExchangeFromQueueViaRoutingKey(exchangeConfig.Name, "non_existing_queue", "routing_key")

		require.NoError(t, err)
	})

	require.NoError(t, manager.DeleteQueue(exchangeConfig.Name))
	require.NoError(t, manager.DeleteQueue(queueConfig.Name))

	require.NoError(t, manager.Disconnect())
}

func (suite *ManagerTestSuite) TestGetNumberOfMessages() {
	t := suite.T()

	t.Setenv("RABBITMQ_HOST", suite.rabbitMQContainer.ContainerHost)
	t.Setenv("RABBITMQ_PORT", strconv.Itoa(int(suite.rabbitMQContainer.ContainerPort)))
	t.Setenv("RABBITMQ_USERNAME", suite.rabbitMQContainer.Username)
	t.Setenv("RABBITMQ_PASSWORD", suite.rabbitMQContainer.Password)

	manager, err := gorabbit.NewManagerFromEnv()

	require.NoError(t, err)
	assert.NotNil(t, manager)

	t.Run("Getting the number of messages from existing queue", func(t *testing.T) {
		queueConfig := gorabbit.QueueConfig{
			Name:      "test_queue",
			Durable:   true,
			Exclusive: false,
		}

		err = manager.CreateQueue(queueConfig)

		require.NoError(t, err)

		exchangeConfig := gorabbit.ExchangeConfig{
			Name: "test_exchange",
			Type: "topic",
		}

		err = manager.CreateExchange(exchangeConfig)

		require.NoError(t, err)

		err = manager.BindExchangeToQueueViaRoutingKey(exchangeConfig.Name, queueConfig.Name, "routing_key")

		require.NoError(t, err)

		count, countErr := manager.GetNumberOfMessages(queueConfig.Name)

		require.NoError(t, countErr)
		assert.Zero(t, count)

		require.NoError(t, manager.DeleteExchange(exchangeConfig.Name))
		require.NoError(t, manager.DeleteQueue(queueConfig.Name))
	})

	t.Run("Getting the number of messages from non-existing queue", func(t *testing.T) {
		count, countErr := manager.GetNumberOfMessages("non_existing_queue")

		require.Error(t, countErr)
		assert.Equal(t, "Exception (404) Reason: \"NOT_FOUND - no queue 'non_existing_queue' in vhost '/'\"", countErr.Error())
		assert.Equal(t, -1, count)
	})

	require.NoError(t, manager.Disconnect())
}

func (suite *ManagerTestSuite) TestPushMessageToExchange() {
	t := suite.T()

	t.Setenv("RABBITMQ_HOST", suite.rabbitMQContainer.ContainerHost)
	t.Setenv("RABBITMQ_PORT", strconv.Itoa(int(suite.rabbitMQContainer.ContainerPort)))
	t.Setenv("RABBITMQ_USERNAME", suite.rabbitMQContainer.Username)
	t.Setenv("RABBITMQ_PASSWORD", suite.rabbitMQContainer.Password)

	manager, err := gorabbit.NewManagerFromEnv()

	require.NoError(t, err)
	assert.NotNil(t, manager)

	t.Run("Push message to exchange", func(t *testing.T) {
		queueConfig := gorabbit.QueueConfig{
			Name:      "test_queue",
			Durable:   true,
			Exclusive: false,
		}

		err = manager.CreateQueue(queueConfig)

		require.NoError(t, err)

		exchangeConfig := gorabbit.ExchangeConfig{
			Name: "test_exchange",
			Type: "topic",
		}

		err = manager.CreateExchange(exchangeConfig)

		require.NoError(t, err)

		err = manager.BindExchangeToQueueViaRoutingKey(exchangeConfig.Name, queueConfig.Name, "routing_key")

		require.NoError(t, err)

		// Push message when the binding exists
		err = manager.PushMessageToExchange(exchangeConfig.Name, "routing_key", "Some message")

		require.NoError(t, err)

		// Small sleep for allowing message to be sent.
		time.Sleep(50 * time.Millisecond)

		count, countErr := manager.GetNumberOfMessages(queueConfig.Name)

		require.NoError(t, countErr)
		assert.Equal(t, 1, count)

		require.NoError(t, manager.PurgeQueue(queueConfig.Name))

		err = manager.UnbindExchangeFromQueueViaRoutingKey(exchangeConfig.Name, queueConfig.Name, "routing_key")

		require.NoError(t, err)

		// Push message when the binding no longer exists
		err = manager.PushMessageToExchange(exchangeConfig.Name, "routing_key", "Some message")

		require.NoError(t, err)

		// Small sleep for allowing message to be sent.
		time.Sleep(50 * time.Millisecond)

		count, countErr = manager.GetNumberOfMessages(queueConfig.Name)

		require.NoError(t, countErr)
		assert.Zero(t, count)

		require.NoError(t, manager.PurgeQueue(queueConfig.Name))

		count, countErr = manager.GetNumberOfMessages(queueConfig.Name)

		require.NoError(t, countErr)
		assert.Zero(t, count)

		require.NoError(t, manager.DeleteExchange(exchangeConfig.Name))
		require.NoError(t, manager.DeleteQueue(queueConfig.Name))
	})

	t.Run("Pushing message to non-existing exchange should still work", func(t *testing.T) {
		err = manager.PushMessageToExchange("non_existing_exchange", "routing_key", "Some message")

		require.NoError(t, err)

		// Small sleep for allowing message to be sent.
		time.Sleep(50 * time.Millisecond)
	})

	require.NoError(t, manager.Disconnect())
}

func (suite *ManagerTestSuite) TestPopMessageFromQueue() {
	t := suite.T()

	t.Setenv("RABBITMQ_HOST", suite.rabbitMQContainer.ContainerHost)
	t.Setenv("RABBITMQ_PORT", strconv.Itoa(int(suite.rabbitMQContainer.ContainerPort)))
	t.Setenv("RABBITMQ_USERNAME", suite.rabbitMQContainer.Username)
	t.Setenv("RABBITMQ_PASSWORD", suite.rabbitMQContainer.Password)

	manager, err := gorabbit.NewManagerFromEnv()

	require.NoError(t, err)
	assert.NotNil(t, manager)

	t.Run("Push message to exchange and consume it", func(t *testing.T) {
		queueConfig := gorabbit.QueueConfig{
			Name:      "test_queue",
			Durable:   true,
			Exclusive: false,
		}

		err = manager.CreateQueue(queueConfig)

		require.NoError(t, err)

		exchangeConfig := gorabbit.ExchangeConfig{
			Name: "test_exchange",
			Type: "topic",
		}

		err = manager.CreateExchange(exchangeConfig)

		require.NoError(t, err)

		err = manager.BindExchangeToQueueViaRoutingKey(exchangeConfig.Name, queueConfig.Name, "routing_key")

		require.NoError(t, err)

		err = manager.PushMessageToExchange(exchangeConfig.Name, "routing_key", "Some message")

		require.NoError(t, err)

		// Small sleep for allowing message to be sent.
		time.Sleep(50 * time.Millisecond)

		count, countErr := manager.GetNumberOfMessages(queueConfig.Name)

		require.NoError(t, countErr)
		assert.Equal(t, 1, count)

		delivery, popErr := manager.PopMessageFromQueue(queueConfig.Name, true)

		require.NoError(t, popErr)
		assert.Equal(t, "\"Some message\"", string(delivery.Body))

		require.NoError(t, manager.PurgeQueue(queueConfig.Name))

		count, countErr = manager.GetNumberOfMessages(queueConfig.Name)

		require.NoError(t, countErr)
		assert.Zero(t, count)

		require.NoError(t, manager.DeleteExchange(exchangeConfig.Name))
		require.NoError(t, manager.DeleteQueue(queueConfig.Name))
	})

	t.Run("Popping message from non-existing queue", func(t *testing.T) {
		delivery, popErr := manager.PopMessageFromQueue("non_existing_queue", true)

		require.Error(t, popErr)
		assert.Nil(t, delivery)
		assert.Equal(t, "Exception (404) Reason: \"NOT_FOUND - no queue 'non_existing_queue' in vhost '/'\"", popErr.Error())
	})

	t.Run("Popping message from existent empty queue", func(t *testing.T) {
		queueConfig := gorabbit.QueueConfig{
			Name:      "test_queue",
			Durable:   true,
			Exclusive: false,
		}

		err = manager.CreateQueue(queueConfig)

		require.NoError(t, err)

		delivery, popErr := manager.PopMessageFromQueue(queueConfig.Name, true)

		require.Error(t, popErr)
		require.Equal(t, "queue is empty", popErr.Error())
		assert.Nil(t, delivery)

		require.NoError(t, manager.DeleteQueue(queueConfig.Name))
	})

	require.NoError(t, manager.Disconnect())
}

func (suite *ManagerTestSuite) TestSetupFromDefinitions() {
	t := suite.T()

	t.Setenv("RABBITMQ_HOST", suite.rabbitMQContainer.ContainerHost)
	t.Setenv("RABBITMQ_PORT", strconv.Itoa(int(suite.rabbitMQContainer.ContainerPort)))
	t.Setenv("RABBITMQ_USERNAME", suite.rabbitMQContainer.Username)
	t.Setenv("RABBITMQ_PASSWORD", suite.rabbitMQContainer.Password)

	manager, err := gorabbit.NewManagerFromEnv()

	require.NoError(t, err)
	assert.NotNil(t, manager)

	t.Run("Setting up from definitions with wrong path", func(t *testing.T) {
		err = manager.SetupFromDefinitions("wrong-path.json")

		require.Error(t, err)
		assert.Equal(t, "open wrong-path.json: no such file or directory", err.Error())
	})

	t.Run("Setting up from definitions with right path", func(t *testing.T) {
		err = manager.SetupFromDefinitions("assets/definitions.example.json")

		require.NoError(t, err)
	})

	require.NoError(t, manager.Disconnect())
}

func TestManagerTestSuite(t *testing.T) {
	suite.Run(t, new(ManagerTestSuite))
}
