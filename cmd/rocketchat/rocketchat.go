package main

import (
	"flag"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/LF-Engineering/insights-datasource-rocketchat/gen/models"
	shared "github.com/LF-Engineering/insights-datasource-shared"
	// jsoniter "github.com/json-iterator/go"
)

const (
	// RocketchatBackendVersion - backend version
	RocketchatBackendVersion = "0.1.0"
)

var (
	gMaxUpstreamDt    time.Time
	gMaxUpstreamDtMtx = &sync.Mutex{}
	// RocketchatDefaultMaxItems - max items to retrieve from API via a single request
	RocketchatDefaultMaxItems = 100
	// RocketchatDefaultMinRate - default min rate points (when not set)
	RocketchatDefaultMinRate = 10
	// MustWaitRE - parse too many requests error message
	MustWaitRE = regexp.MustCompile(`must wait (\d+) seconds before`)
	// RocketchatDataSource - constant
	RocketchatDataSource = &models.DataSource{Name: "RocketChat", Slug: "rocketchat"}
	gRocketchatMetaData  = &models.MetaData{BackendName: "rocketchat", BackendVersion: RocketchatBackendVersion}
)

// DSRocketchat - DS implementation for rocketchat - does nothing at all, just presents a skeleton code
type DSRocketchat struct {
	URL          string // rocketchat server url
	Channel      string // rocketchat channel
	User         string // user name
	Token        string // token
	MaxItems     int    // max items to retrieve from API via a single request - defaults to 100
	MinRate      int    // min API points, if we reach this value we wait for refresh, default 10
	WaitRate     bool   // will wait for rate limit refresh if set, otherwise will fail is rate limit is reached
	FlagURL      *string
	FlagChannel  *string
	FlagUser     *string
	FlagToken    *string
	FlagMaxItems *int
	FlagMinRate  *int
	FlagWaitRate *bool
}

// AddFlags - add RocketChat specific flags
func (j *DSRocketchat) AddFlags() {
	j.FlagURL = flag.String("rocketchat-url", "", "RocketChat server URL, for example https://chat.hyperledger.org")
	j.FlagChannel = flag.String("rocketchat-channel", "", "RocketChat channel, for example sawtooth")
	j.FlagUser = flag.String("rocketchat-user", "", "User: API user ID")
	j.FlagToken = flag.String("rocketchat-token", "", "Token: API token")
	j.FlagMaxItems = flag.Int("rocketchat-", RocketchatDefaultMaxItems, "max items to retrieve from API via a single request - defaults to 100")
	j.FlagMinRate = flag.Int("rocketchat-", RocketchatDefaultMinRate, "min API points, if we reach this value we wait for refresh, default 10")
	j.FlagWaitRate = flag.Bool("rocketchat-", false, "will wait for rate limit refresh if set, otherwise will fail is rate limit is reached")
}

// ParseArgs - parse rocketchat specific environment variables
func (j *DSRocketchat) ParseArgs(ctx *shared.Ctx) (err error) {
	// RocketChat Server URL
	if shared.FlagPassed(ctx, "url") && *j.FlagURL != "" {
		j.URL = *j.FlagURL
	}
	if ctx.EnvSet("URL") {
		j.URL = ctx.Env("URL")
	}

	// RocketChat channel
	if shared.FlagPassed(ctx, "channel") && *j.FlagChannel != "" {
		j.Channel = *j.FlagChannel
	}
	if ctx.EnvSet("CHANNEL") {
		j.Channel = ctx.Env("CHANNEL")
	}

	// User
	if shared.FlagPassed(ctx, "user") && *j.FlagUser != "" {
		j.User = *j.FlagUser
	}
	if ctx.EnvSet("USER") {
		j.User = ctx.Env("USER")
	}
	if j.User != "" {
		shared.AddRedacted(j.User, false)
	}

	// Token
	if shared.FlagPassed(ctx, "token") && *j.FlagToken != "" {
		j.Token = *j.FlagToken
	}
	if ctx.EnvSet("TOKEN") {
		j.Token = ctx.Env("TOKEN")
	}
	if j.Token != "" {
		shared.AddRedacted(j.Token, false)
	}

	// Max items
	passed := shared.FlagPassed(ctx, "max-items")
	if passed {
		j.MaxItems = *j.FlagMaxItems
	}
	if ctx.EnvSet("MAX_ITEMS") {
		maxItems, err := strconv.Atoi(ctx.Env("MAX_ITEMS"))
		shared.FatalOnError(err)
		if maxItems > 0 {
			j.MaxItems = maxItems
		}
	} else if !passed {
		j.MaxItems = RocketchatDefaultMaxItems
	}

	// Min rate
	passed = shared.FlagPassed(ctx, "min-rate")
	if passed {
		j.MinRate = *j.FlagMinRate
	}
	if ctx.EnvSet("MIN_RATE") {
		minRate, err := strconv.Atoi(ctx.Env("MIN_RATE"))
		shared.FatalOnError(err)
		if minRate > 0 {
			j.MinRate = minRate
		}
	} else if !passed {
		j.MinRate = RocketchatDefaultMinRate
	}

	// Wait Rate
	if shared.FlagPassed(ctx, "wait-rate") {
		j.WaitRate = *j.FlagWaitRate
	}
	waitRate, present := ctx.BoolEnvSet("WAIT_RATE")
	if present {
		j.WaitRate = waitRate
	}

	// NOTE: don't forget this
	gRocketchatMetaData.Project = ctx.Project
	gRocketchatMetaData.Tags = ctx.Tags
	return
}

// Validate - is current DS configuration OK?
func (j *DSRocketchat) Validate() (err error) {
	j.URL = strings.TrimSpace(j.URL)
	if strings.HasSuffix(j.URL, "/") {
		j.URL = j.URL[:len(j.URL)-1]
	}
	j.Channel = strings.TrimSpace(j.Channel)
	if j.URL == "" || j.Channel == "" || j.User == "" || j.Token == "" {
		err = fmt.Errorf("URL, Channel, User, Token must all be set")
	}
	return
}

// Endpoint - return unique endpoint string representation
func (j *DSRocketchat) Endpoint() string {
	return j.URL + " " + j.Channel
}

// Init - initialize RocketChat data source
func (j *DSRocketchat) Init(ctx *shared.Ctx) (err error) {
	shared.NoSSLVerify()
	ctx.InitEnv("RocketChat")
	j.AddFlags()
	ctx.Init()
	err = j.ParseArgs(ctx)
	if err != nil {
		return
	}
	err = j.Validate()
	if err != nil {
		return
	}
	if ctx.Debug > 1 {
		m := &models.Data{}
		shared.Printf("RocketChat: %+v\nshared context: %s\nModel: %+v", j, ctx.Info(), m)
	}
	return
}

// Sync - sync rocketchat data source
func (j *DSRocketchat) Sync(ctx *shared.Ctx) (err error) {
	thrN := shared.GetThreadsNum(ctx)
	if ctx.DateFrom == nil {
		ctx.DateFrom = shared.GetLastUpdate(ctx, j.URL)
	}
	if ctx.DateFrom != nil {
		shared.Printf("%s resuming from %v (%d threads)\n", j.Endpoint(), ctx.DateFrom, thrN)
	}
	if ctx.DateTo != nil {
		shared.Printf("%s fetching till %v (%d threads)\n", j.Endpoint(), ctx.DateTo, thrN)
	}
	// NOTE: Non-generic starts here
	// NOTE: Non-generic ends here
	gMaxUpstreamDtMtx.Lock()
	defer gMaxUpstreamDtMtx.Unlock()
	shared.SetLastUpdate(ctx, j.Endpoint(), gMaxUpstreamDt)
	return
}

func main() {
	var (
		ctx        shared.Ctx
		rocketchat DSRocketchat
	)
	err := rocketchat.Init(&ctx)
	if err != nil {
		shared.Printf("Error: %+v\n", err)
		return
	}
	err = rocketchat.Sync(&ctx)
	if err != nil {
		shared.Printf("Error: %+v\n", err)
		return
	}
}
