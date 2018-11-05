package log

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"testing"
	"time"

	"github.com/TerrexTech/uuuid"

	"github.com/TerrexTech/go-eventstore-models/model"

	"github.com/Shopify/sarama"
	"github.com/TerrexTech/go-commonutils/commonutil"
	"github.com/TerrexTech/go-kafkautils/kafka"
	"github.com/joho/godotenv"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
)

func TestLogSink(t *testing.T) {
	log.Println("Reading environment file")
	err := godotenv.Load("../test.env")
	if err != nil {
		err = errors.Wrap(err,
			".env file not found, env-vars will be read as set in environment",
		)
		log.Println(err)
	}

	missingVar, err := commonutil.ValidateEnv(
		"KAFKA_BROKERS",
		"KAFKA_LOG_CONSUMER_GROUP",
		"KAFKA_LOG_PRODUCER_TOPIC",
	)
	if err != nil {
		err = errors.Wrapf(err, `Env-var "%s" is required, but is not set`, missingVar)
		log.Fatalln(err)
	}

	RegisterFailHandler(Fail)
	RunSpecs(t, "LogSink Suite")
}

// Log-message handler for testing
type msgHandler struct {
	msgCallback func(*sarama.ConsumerMessage) bool
}

func (*msgHandler) Setup(sarama.ConsumerGroupSession) error {
	log.Println("Initializing Kafka MsgHandler")
	return nil
}

func (*msgHandler) Cleanup(sarama.ConsumerGroupSession) error {
	log.Println("Closing Kafka MsgHandler")
	return nil
}

func (m *msgHandler) ConsumeClaim(
	session sarama.ConsumerGroupSession,
	claim sarama.ConsumerGroupClaim,
) error {
	if m.msgCallback == nil {
		return errors.New("msgCallback cannot be nil")
	}
	for msg := range claim.Messages() {
		session.MarkMessage(msg, "")

		val := m.msgCallback(msg)
		if val {
			return nil
		}
	}
	return errors.New("required value not found")
}

var _ = Describe("LogSink", func() {
	var kafkaBrokers []string

	BeforeSuite(func() {
		kafkaBrokersStr := os.Getenv("KAFKA_BROKERS")
		kafkaBrokers = *commonutil.ParseHosts(kafkaBrokersStr)
	})

	Describe("test log-production", func() {
		var (
			logger LoggerI

			consumer *kafka.Consumer
			topic    string
		)

		BeforeEach(func() {
			cGroup := os.Getenv("KAFKA_LOG_CONSUMER_GROUP")
			topic = os.Getenv("KAFKA_LOG_PRODUCER_TOPIC")

			var err error

			prodConfig := &kafka.ProducerConfig{
				KafkaBrokers: kafkaBrokers,
			}
			ctx := context.Background()
			logger, err = Init(ctx, "testsvc", prodConfig, topic)
			Expect(err).ToNot(HaveOccurred())

			consumer, err = kafka.NewConsumer(&kafka.ConsumerConfig{
				GroupName:    cGroup,
				KafkaBrokers: kafkaBrokers,
				Topics:       []string{topic},
			})
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := consumer.Close()
			Expect(err).ToNot(HaveOccurred())
		})

		It("should produce logs", func(done Done) {
			go func() {
				for err := range consumer.Errors() {
					defer GinkgoRecover()
					Expect(err).ToNot(HaveOccurred())
				}
			}()

			testLog := Entry{
				Description:   "test-log",
				ErrorCode:     0,
				EventAction:   "test-eventaction",
				ServiceAction: "test-svcaction",
				ServiceName:   "testsvc",
			}
			logger.I(testLog)

			msgCallback := func(msg *sarama.ConsumerMessage) bool {
				defer GinkgoRecover()
				log.Println("A Response was received on response channel")

				l := &Entry{}
				err := json.Unmarshal(msg.Value, l)
				Expect(err).ToNot(HaveOccurred())

				if *l == testLog {
					log.Println("The response matches")
					close(done)
					return true
				}
				return false
			}

			handler := &msgHandler{msgCallback}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			consumer.Consume(ctx, handler)
		}, 20)

		It("should use default service-name if none was provided", func(done Done) {
			go func() {
				for err := range consumer.Errors() {
					defer GinkgoRecover()
					Expect(err).ToNot(HaveOccurred())
				}
			}()

			testLog := Entry{
				Description:   "test-log",
				ErrorCode:     0,
				EventAction:   "test-eventaction",
				ServiceAction: "test-svcaction",
			}
			logger.I(testLog)
			testLog.ServiceName = "testsvc"

			msgCallback := func(msg *sarama.ConsumerMessage) bool {
				defer GinkgoRecover()
				log.Println("A Response was received on response channel")

				l := &Entry{}
				err := json.Unmarshal(msg.Value, l)
				Expect(err).ToNot(HaveOccurred())

				if *l == testLog {
					log.Println("The response matches")
					close(done)
					return true
				}
				return false
			}

			handler := &msgHandler{msgCallback}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			consumer.Consume(ctx, handler)
		})

		It("should not publish DEBUG logs if INFO level is specified", func() {
			go func() {
				for err := range consumer.Errors() {
					Expect(err).To(HaveOccurred())
				}
			}()

			err := os.Setenv(LogLevelEnvVar, "INFO")
			Expect(err).ToNot(HaveOccurred())

			uuid1, err := uuuid.NewV4()
			Expect(err).ToNot(HaveOccurred())
			logger.I(Entry{
				Description:   uuid1.String(),
				ErrorCode:     0,
				EventAction:   uuid1.String(),
				ServiceAction: "test-svcaction",
				ServiceName:   "some-name",
			})

			uuid2, err := uuuid.NewV4()
			Expect(err).ToNot(HaveOccurred())
			logger.E(Entry{
				Description:   uuid2.String(),
				ErrorCode:     0,
				EventAction:   uuid2.String(),
				ServiceAction: "test-svcaction",
				ServiceName:   "some-name",
			})

			uuid3, err := uuuid.NewV4()
			Expect(err).ToNot(HaveOccurred())
			logger.D(Entry{
				Description:   uuid3.String(),
				ErrorCode:     0,
				EventAction:   uuid3.String(),
				ServiceAction: "test-svcaction",
				ServiceName:   "some-name",
			})

			dSuccess := true
			eSuccess := false
			iSuccess := false
			msgCallback := func(msg *sarama.ConsumerMessage) bool {
				defer GinkgoRecover()

				l := &Entry{}
				err := json.Unmarshal(msg.Value, l)
				Expect(err).ToNot(HaveOccurred())

				switch l.Description {
				case uuid3.String():
					dSuccess = false
				case uuid2.String():
					eSuccess = true
				case uuid1.String():
					iSuccess = true
				}
				if !dSuccess {
					return true
				}
				return false
			}

			handler := &msgHandler{msgCallback}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			consumer.Consume(ctx, handler)

			Expect(dSuccess).To(BeTrue())
			Expect(eSuccess).To(BeTrue())
			Expect(iSuccess).To(BeTrue())
		})

		It("should not publish DEBUG and INFO logs if ERROR level is specified", func() {
			go func() {
				for err := range consumer.Errors() {
					Expect(err).To(HaveOccurred())
				}
			}()

			err := os.Setenv(LogLevelEnvVar, "ERROR")
			Expect(err).ToNot(HaveOccurred())

			uuid1, err := uuuid.NewV4()
			Expect(err).ToNot(HaveOccurred())
			logger.I(Entry{
				Description:   uuid1.String(),
				ErrorCode:     0,
				EventAction:   uuid1.String(),
				ServiceAction: "test-svcaction",
				ServiceName:   "some-name",
			})

			uuid2, err := uuuid.NewV4()
			Expect(err).ToNot(HaveOccurred())
			logger.D(Entry{
				Description:   uuid2.String(),
				ErrorCode:     0,
				EventAction:   uuid2.String(),
				ServiceAction: "test-svcaction",
				ServiceName:   "some-name",
			})

			isMsgReceived := false
			msgCallback := func(msg *sarama.ConsumerMessage) bool {
				defer GinkgoRecover()

				l := &Entry{}
				err := json.Unmarshal(msg.Value, l)
				Expect(err).ToNot(HaveOccurred())

				if l.Description == uuid1.String() || l.Description == uuid2.String() {
					isMsgReceived = true
					return true
				}
				return false
			}

			handler := &msgHandler{msgCallback}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			consumer.Consume(ctx, handler)

			Expect(isMsgReceived).To(BeFalse())
		})

		It("should not publish logs if NONE level is specified", func() {
			go func() {
				for err := range consumer.Errors() {
					Expect(err).To(HaveOccurred())
				}
			}()

			uuid, err := uuuid.NewV4()
			Expect(err).ToNot(HaveOccurred())

			err = os.Setenv(LogLevelEnvVar, "NONE")
			Expect(err).ToNot(HaveOccurred())
			testLog := Entry{
				Description:   uuid.String(),
				ErrorCode:     0,
				EventAction:   uuid.String(),
				ServiceAction: "test-svcaction",
				ServiceName:   "some-name",
			}
			logger.I(testLog)

			isMsgReceived := false
			msgCallback := func(msg *sarama.ConsumerMessage) bool {
				defer GinkgoRecover()

				l := &Entry{}
				err := json.Unmarshal(msg.Value, l)
				Expect(err).ToNot(HaveOccurred())

				if l.Description == testLog.Description {
					isMsgReceived = true
					return true
				}
				return false
			}

			handler := &msgHandler{msgCallback}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			consumer.Consume(ctx, handler)

			Expect(isMsgReceived).To(BeFalse())
		})

		It("should add data to log-entry if DEBUG level is specified", func(done Done) {
			go func() {
				for err := range consumer.Errors() {
					defer GinkgoRecover()
					Expect(err).ToNot(HaveOccurred())
				}
			}()

			uuid, err := uuuid.NewV4()
			Expect(err).ToNot(HaveOccurred())

			err = os.Setenv(LogLevelEnvVar, "DEBUG")
			Expect(err).ToNot(HaveOccurred())
			testLog := Entry{
				Description:   uuid.String(),
				ErrorCode:     0,
				EventAction:   uuid.String(),
				ServiceAction: "test-svcaction",
				ServiceName:   "some-name",
			}
			t1 := &model.Event{
				EventAction: "test-action",
				Data:        []byte("some-data"),
			}
			t2 := &model.EventStoreQuery{
				AggregateID:      1,
				AggregateVersion: 3,
			}
			t3 := []model.EventMeta{
				model.EventMeta{
					AggregateID:      1,
					AggregateVersion: 3,
				},
				model.EventMeta{
					AggregateID:      2,
					AggregateVersion: 8,
				},
			}
			t4 := model.KafkaResponse{
				AggregateID: 1,
				EventAction: "testaction",
			}
			logger.D(testLog, t1, t2, t3, t4, "testData5", 4)

			desc, err := fmtDebug(testLog.Description, t1, t2, t3, t4, "testData5", 4)
			Expect(err).ToNot(HaveOccurred())
			testLog.Description = desc

			msgCallback := func(msg *sarama.ConsumerMessage) bool {
				defer GinkgoRecover()
				log.Println("A Response was received on response channel")

				l := Entry{}
				err := json.Unmarshal(msg.Value, &l)
				Expect(err).ToNot(HaveOccurred())

				if l == testLog {
					log.Println("The response matches")
					close(done)
					return true
				}
				return false
			}

			handler := &msgHandler{msgCallback}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			consumer.Consume(ctx, handler)
		}, 20)
	})

	It("should use background-context when nil context is provided", func() {
		prodConfig := &kafka.ProducerConfig{
			KafkaBrokers: kafkaBrokers,
		}
		_, err := Init(context.Background(), "testsvc", prodConfig, "test-topic")
		Expect(err).ToNot(HaveOccurred())
	})

	It("should return error if default svc-name is empty", func() {
		_, err := Init(context.Background(), "", &kafka.ProducerConfig{}, "")
		Expect(err).To(HaveOccurred())
	})

	It("should return error if producer-config is nil", func() {
		_, err := Init(context.Background(), "testsvc", nil, "test-topic")
		Expect(err).To(HaveOccurred())
	})

	It("should return error if producer-topic is empty", func() {
		_, err := Init(context.Background(), "testsvc", &kafka.ProducerConfig{}, "")
		Expect(err).To(HaveOccurred())
	})
})
