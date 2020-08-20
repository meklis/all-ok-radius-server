package radius

import (
	"fmt"
	rad_api "github.com/meklis/all-ok-radius-server/api"
	"github.com/meklis/all-ok-radius-server/logger"
	"layeh.com/radius"
	"log"
	"os"
	"sync"
	"time"
)

type Radius struct {
	lg           *logger.Logger
	listenAddr   string
	secret       string
	agentParsing bool
	api          *rad_api.Api
	classId      int64
	sync.Mutex
}

func Init() *Radius {
	rad := new(Radius)
	rad.listenAddr = "0.0.0.0:1812"
	rad.secret = "secret"
	rad.agentParsing = true
	rad.lg, _ = logger.New("radius", 0, os.Stdout)
	rad.classId = time.Now().Unix()
	return rad
}

func (rad *Radius) getClassId() string {
	rad.Lock()
	defer rad.Unlock()
	rad.classId = rad.classId + 1
	return fmt.Sprintf("%0x", rad.classId)
}

func (rad *Radius) SetLogger(lg *logger.Logger) *Radius {
	rad.lg = lg
	return rad
}
func (rad *Radius) SetListenAddr(listenAddr string) *Radius {
	rad.listenAddr = listenAddr
	return rad
}
func (rad *Radius) SetSecret(secret string) *Radius {
	rad.secret = secret
	return rad
}
func (rad *Radius) SetAgentParsing(enabled bool) *Radius {
	rad.agentParsing = enabled
	return rad
}

func (rad *Radius) SetAPI(apiR *rad_api.Api) *Radius {
	rad.api = apiR
	return rad
}

func (rad *Radius) ListenAndServe() error {
	server := radius.PacketServer{
		Addr:         rad.listenAddr,
		Network:      "udp",
		SecretSource: radius.StaticSecretSource([]byte(rad.secret)),
		Handler:      radius.HandlerFunc(rad.handler),
	}

	rad.lg.InfoF("Starting radius server on %v", rad.listenAddr)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
		return err
	}
	return nil
}
