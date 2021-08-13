package main

import (
	"flag"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	neturl "net/url"

	"github.com/LF-Engineering/insights-datasource-rocketchat/gen/models"
	shared "github.com/LF-Engineering/insights-datasource-shared"
	jsoniter "github.com/json-iterator/go"
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
	// RocketchatDefaultSearchField - default search field
	RocketchatDefaultSearchField = "item_id"
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

// CalculateTimeToReset - calculate time to reset rate limits based on rate limit value and rate limit reset value
func (j *DSRocketchat) CalculateTimeToReset(ctx *shared.Ctx, rateLimit, rateLimitReset int) (seconds int) {
	seconds = (int(int64(rateLimitReset)-(time.Now().UnixNano()/int64(1000000))) / 1000) + 1
	if seconds < 0 {
		seconds = 0
	}
	if ctx.Debug > 1 {
		shared.Printf("CalculateTimeToReset(%d,%d) -> %d\n", rateLimit, rateLimitReset, seconds)
	}
	return
}

// UpdateRateLimit - generic function to get rate limit data from header
func (j *DSRocketchat) UpdateRateLimit(ctx *shared.Ctx, headers map[string][]string, rateLimitHeader, rateLimitResetHeader string) (rateLimit, rateLimitReset, secondsToReset int) {
	if rateLimitHeader == "" {
		rateLimitHeader = shared.DefaultRateLimitHeader
	}
	if rateLimitResetHeader == "" {
		rateLimitResetHeader = shared.DefaultRateLimitResetHeader
	}
	v, ok := headers[rateLimitHeader]
	if !ok {
		lRateLimitHeader := strings.ToLower(rateLimitHeader)
		for k, va := range headers {
			kl := strings.ToLower(k)
			if kl == lRateLimitHeader {
				v = va
				ok = true
				break
			}
		}
	}
	if ok {
		if len(v) > 0 {
			rateLimit, _ = strconv.Atoi(v[0])
		}
	}
	v, ok = headers[rateLimitResetHeader]
	if !ok {
		lRateLimitResetHeader := strings.ToLower(rateLimitResetHeader)
		for k, va := range headers {
			kl := strings.ToLower(k)
			if kl == lRateLimitResetHeader {
				v = va
				ok = true
				break
			}
		}
	}
	if ok {
		if len(v) > 0 {
			var err error
			rateLimitReset, err = strconv.Atoi(v[0])
			if err == nil {
				secondsToReset = j.CalculateTimeToReset(ctx, rateLimit, rateLimitReset)
			}
		}
	}
	if ctx.Debug > 1 {
		shared.Printf("UpdateRateLimit(%+v,%s,%s) --> (%d,%d,%d)\n", headers, rateLimitHeader, rateLimitResetHeader, rateLimit, rateLimitReset, secondsToReset)
	}
	return
}

// SleepForRateLimit - sleep for rate or return error when rate exceeded
func (j *DSRocketchat) SleepForRateLimit(ctx *shared.Ctx, rateLimit, rateLimitReset, minRate int, waitRate bool) (err error) {
	if rateLimit <= 0 || rateLimit > minRate {
		if ctx.Debug > 1 {
			shared.Printf("rate limit is %d, min rate is %d, no need to wait\n", rateLimit, minRate)
		}
		return
	}
	secondsToReset := j.CalculateTimeToReset(ctx, rateLimit, rateLimitReset)
	if secondsToReset < 0 {
		shared.Printf("Warning: time to reset is negative %d, resetting to 0\n", secondsToReset)
		secondsToReset = 0
	}
	if waitRate && secondsToReset > 0 {
		// Give one more second
		secondsToReset++
		shared.Printf("Waiting %d seconds for rate limit reset.\n", secondsToReset)
		time.Sleep(time.Duration(secondsToReset) * time.Second)
		shared.Printf("Waited %d seconds for rate limit reset.\n", secondsToReset)
		return
	}
	err = fmt.Errorf("rate limit exceeded, not waiting %d seconds", secondsToReset)
	return
}

// SleepAsRequested - parse server's:
// {"success":false,"error":"Error, too many requests. Please slow down. You must wait 23 seconds before trying this endpoint again. [error-too-many-requests]"}
// And sleep N+1 requested seconds
func (j *DSRocketchat) SleepAsRequested(res interface{}, thrN int) {
	iErrorMsg, ok := res.(map[string]interface{})["error"]
	if !ok {
		shared.Printf("Unable to parse sleep duration, assuming 1m\n")
		time.Sleep(time.Duration(60) * time.Second)
		return
	}
	errorMsg, _ := iErrorMsg.(string)
	match := MustWaitRE.FindAllStringSubmatch(errorMsg, -1)
	if len(match) < 1 {
		shared.Printf("Unable to parse sleep duration from '%s', assuming 1m\n", errorMsg)
		time.Sleep(time.Duration(60) * time.Second)
		return
	}
	sleepFor, _ := strconv.Atoi(match[0][1])
	sleepFor++
	sleepFor *= thrN
	shared.Printf("Sleeping for %d (adjusted for MT) seconds, as requested in '%s'\n", sleepFor, errorMsg)
	time.Sleep(time.Duration(sleepFor) * time.Second)
}

// ItemID - return unique identifier for an item
func (j *DSRocketchat) ItemID(item interface{}) string {
	id, _ := shared.Dig(item, []string{"_id"}, true, false)
	return id.(string)
}

// ItemUpdatedOn - return updated on date for an item
func (j *DSRocketchat) ItemUpdatedOn(item interface{}) time.Time {
	iUpdated, _ := shared.Dig(item, []string{"_updatedAt"}, true, false)
	updated, err := shared.TimeParseAny(iUpdated.(string))
	shared.FatalOnError(err)
	return updated
}

// AddMetadata - add metadata to the item
func (j *DSRocketchat) AddMetadata(ctx *shared.Ctx, item interface{}) (mItem map[string]interface{}) {
	mItem = make(map[string]interface{})
	origin := j.Endpoint()
	tags := ctx.Tags
	if len(tags) == 0 {
		tags = []string{origin}
	}
	itemID := j.ItemID(item)
	updatedOn := j.ItemUpdatedOn(item)
	uuid := shared.UUIDNonEmpty(ctx, origin, itemID)
	timestamp := time.Now()
	mItem["backend_name"] = ctx.DS
	mItem["backend_version"] = RocketchatBackendVersion
	mItem["timestamp"] = fmt.Sprintf("%.06f", float64(timestamp.UnixNano())/1.0e9)
	mItem["uuid"] = uuid
	mItem["origin"] = origin
	mItem["tags"] = tags
	mItem["offset"] = float64(updatedOn.Unix())
	mItem["category"] = "message"
	mItem["search_fields"] = make(map[string]interface{})
	channelID, _ := shared.Dig(item, []string{"channel_info", "_id"}, true, false)
	channelName, _ := shared.Dig(item, []string{"channel_info", "name"}, true, false)
	shared.FatalOnError(shared.DeepSet(mItem, []string{"search_fields", RocketchatDefaultSearchField}, itemID, false))
	shared.FatalOnError(shared.DeepSet(mItem, []string{"search_fields", "channel_id"}, channelID, false))
	shared.FatalOnError(shared.DeepSet(mItem, []string{"search_fields", "channel_name"}, channelName, false))
	mItem["metadata__updated_on"] = shared.ToESDate(updatedOn)
	mItem["metadata__timestamp"] = shared.ToESDate(timestamp)
	// mItem[ProjectSlug] = ctx.ProjectSlug
	return
}

// EnrichItem - return rich item from raw item for a given author type
func (j *DSRocketchat) EnrichItem(ctx *shared.Ctx, item map[string]interface{}) (rich map[string]interface{}, err error) {
	// FIXME
	jsonBytes, _ := jsoniter.Marshal(item)
	shared.Printf("%s\n", string(jsonBytes))
	rich = make(map[string]interface{})
	// FIXME
	/*
		for _, field := range RawFields {
			v, _ := item[field]
			rich[field] = v
		}
		message, ok := item["data"].(map[string]interface{})
		if !ok {
			err = fmt.Errorf("missing data field in item %+v", DumpKeys(item))
			return
		}
		msg, _ := message["msg"]
		rich["msg_analyzed"] = msg
		rich["msg"] = msg
		rich["rid"], _ = message["rid"]
		rich["msg_id"], _ = message["_id"]
		rich["msg_parent"], _ = message["parent"]
		iAuthor, ok := message["u"]
		if ok {
			author, _ := iAuthor.(map[string]interface{})
			rich["user_id"], _ = author["_id"]
			rich["user_name"], _ = author["name"]
			rich["user_username"], _ = author["username"]
		}
		rich["is_edited"] = 0
		iEditor, ok := message["editedBy"]
		if ok {
			editor, _ := iEditor.(map[string]interface{})
			iEdited, ok := editor["editedAt"]
			if ok {
				edited, err := TimeParseAny(iEdited.(string))
				if err == nil {
					rich["edited_at"] = edited
				}
			}
			rich["edited_by_username"], _ = editor["username"]
			rich["edited_by_user_id"], _ = editor["_id"]
			rich["is_edited"] = 1
		}
		iFile, ok := message["file"]
		if ok {
			file, _ := iFile.(map[string]interface{})
			rich["file_id"], _ = file["_id"]
			rich["file_name"], _ = file["name"]
			rich["file_type"], _ = file["type"]
		}
		iReplies, ok := message["replies"]
		if ok {
			replies, ok := iReplies.([]interface{})
			if ok {
				rich["replies"] = len(replies)
			} else {
				rich["replies"] = 0
			}
		} else {
			rich["replies"] = 0
		}
		rich["total_reactions"] = 0
		iReactions, ok := message["reactions"]
		if ok {
			reactions, _ := iReactions.(map[string]interface{})
			rich["reactions"], rich["total_reactions"] = j.GetReactions(reactions)
		}
		rich["total_mentions"] = 0
		iMentions, ok := message["mentions"]
		if ok {
			mentions, _ := iMentions.([]interface{})
			mentionsAry := j.GetMentions(mentions)
			rich["mentions"] = mentionsAry
			rich["total_mentions"] = len(mentionsAry)
		}
		iChannelInfo, ok := message["channel_info"]
		if ok {
			channelInfo, _ := iChannelInfo.(map[string]interface{})
			j.SetChannelInfo(rich, channelInfo)
		}
		rich["total_urls"] = 0
		iURLs, ok := message["urls"]
		if ok {
			urls, _ := iURLs.([]interface{})
			urlsAry := []interface{}{}
			for _, iURL := range urls {
				url, _ := iURL.(map[string]interface{})
				urlsAry = append(urlsAry, url["url"])
			}
			rich["message_urls"] = urlsAry
			rich["total_urls"] = len(urlsAry)
		}
		updatedOn, _ := Dig(item, []string{j.DateField(ctx)}, true, false)
		if affs {
			authorKey := "u"
			var affsItems map[string]interface{}
			affsItems, err = j.AffsItems(ctx, item, RocketchatRoles, updatedOn)
			if err != nil {
				return
			}
			for prop, value := range affsItems {
				rich[prop] = value
			}
			for _, suff := range AffsFields {
				rich[Author+suff] = rich[authorKey+suff]
			}
			orgsKey := authorKey + MultiOrgNames
			_, ok := Dig(rich, []string{orgsKey}, false, true)
			if !ok {
				rich[orgsKey] = []interface{}{}
			}
		}
		for prop, value := range CommonFields(j, updatedOn, Message) {
			rich[prop] = value
		}
	*/
	return
}

// GetModelData - return data in swagger format
func (j *DSRocketchat) GetModelData(ctx *shared.Ctx, docs []interface{}) (data *models.Data) {
	//endpoint := j.Endpoint()
	data = &models.Data{
		DataSource: RocketchatDataSource,
		MetaData:   gRocketchatMetaData,
		Endpoint: &models.DataEndpoint{
			RocketChatServer:  j.URL,
			RocketChatChannel: j.Channel,
		},
	}
	//source := data.DataSource.Slug
	// FIXME
	return
}

// RocketchatEnrichItems - iterate items and enrich them
// items is a current pack of input items
// docs is a pointer to where extracted identities will be stored
func (j *DSRocketchat) RocketchatEnrichItems(ctx *shared.Ctx, thrN int, items []interface{}, docs *[]interface{}, final bool) (err error) {
	shared.Printf("input processing(%d/%d/%v)\n", len(items), len(*docs), final)
	outputDocs := func() {
		if len(*docs) > 0 {
			// actual output
			shared.Printf("output processing(%d/%d/%v)\n", len(items), len(*docs), final)
			data := j.GetModelData(ctx, *docs)
			// FIXME: actual output to some consumer...
			jsonBytes, err := jsoniter.Marshal(data)
			if err != nil {
				shared.Printf("Error: %+v\n", err)
				return
			}
			shared.Printf("%s\n", string(jsonBytes))
			*docs = []interface{}{}
			gMaxUpstreamDtMtx.Lock()
			defer gMaxUpstreamDtMtx.Unlock()
			shared.SetLastUpdate(ctx, j.Endpoint(), gMaxUpstreamDt)
		}
	}
	if final {
		defer func() {
			outputDocs()
		}()
	}
	// NOTE: non-generic code starts
	if ctx.Debug > 0 {
		shared.Printf("rocketchat enrich items %d/%d func\n", len(items), len(*docs))
	}
	var (
		mtx *sync.RWMutex
		ch  chan error
	)
	if thrN > 1 {
		mtx = &sync.RWMutex{}
		ch = make(chan error)
	}
	nThreads := 0
	procItem := func(c chan error, idx int) (e error) {
		if thrN > 1 {
			mtx.RLock()
		}
		item := items[idx]
		if thrN > 1 {
			mtx.RUnlock()
		}
		defer func() {
			if c != nil {
				c <- e
			}
		}()
		// NOTE: never refer to _source - we no longer use ES
		doc, ok := item.(map[string]interface{})
		if !ok {
			e = fmt.Errorf("Failed to parse document %+v", doc)
			return
		}
		// Actual item enrichment
		var rich map[string]interface{}
		rich, e = j.EnrichItem(ctx, doc)
		if e != nil {
			return
		}
		if thrN > 1 {
			mtx.Lock()
		}
		*docs = append(*docs, rich)
		if thrN > 1 {
			mtx.Unlock()
		}
		return
	}
	if thrN > 1 {
		for i := range items {
			go func(i int) {
				_ = procItem(ch, i)
			}(i)
			nThreads++
			if nThreads == thrN {
				err = <-ch
				if err != nil {
					return
				}
				nThreads--
			}
		}
		for nThreads > 0 {
			err = <-ch
			nThreads--
			if err != nil {
				return
			}
		}
		return
	}
	for i := range items {
		err = procItem(nil, i)
		if err != nil {
			return
		}
	}
	return
}

// GetRocketchatMessages - get confluence historical contents
func (j *DSRocketchat) GetRocketchatMessages(ctx *shared.Ctx, fromDate, toDate string, offset, rateLimit, rateLimitReset, thrN int) (messages []map[string]interface{}, newOffset, total, outRateLimit, outRateLimitReset int, err error) {
	// query := `{"_updatedAt": {"$gte": {"$date": "` + fromDate + `"}}}`
	query := `{"_updatedAt":{"$and":[{"$gte":{"$date":"` + fromDate + `"}},{"$lt":{"$date": "` + toDate + `"}}]}}`
	url := j.URL + fmt.Sprintf(
		`/api/v1/channels.messages?roomName=%s&count=%d&offset=%d&sort=%s&query=%s`,
		neturl.QueryEscape(j.Channel),
		j.MaxItems,
		offset,
		neturl.QueryEscape(`{"_updatedAt": 1}`),
		neturl.QueryEscape(query),
	)
	// Let's cache messages for 1 hour (so there are no rate limit hits during the development)
	// FIXME
	cacheDur := time.Duration(8) * time.Hour
	// cacheDur := time.Duration(1) * time.Hour
	method := "GET"
	headers := map[string]string{"X-User-ID": j.User, "X-Auth-Token": j.Token}
	//Printf("%s %+v\n", method, headers)
	//Printf("URL: %s\n", url)
	var (
		res        interface{}
		status     int
		outHeaders map[string][]string
	)
	sleeps, rates := 0, 0
	for {
		err = j.SleepForRateLimit(ctx, rateLimit, rateLimitReset, j.MinRate, j.WaitRate)
		if err != nil {
			return
		}
		res, status, _, outHeaders, err = shared.Request(
			ctx,
			url,
			method,
			headers,
			nil,
			nil,
			map[[2]int]struct{}{{200, 200}: {}, {429, 429}: {}}, // JSON statuses: 200, 429
			nil, // Error statuses
			map[[2]int]struct{}{{200, 200}: {}, {429, 429}: {}}, // OK statuses: 200, 429
			map[[2]int]struct{}{{200, 200}: {}},                 // Cache statuses: 200
			true,                                                // retry
			&cacheDur,                                           // cache duration
			false,                                               // skip in dry-run mode
		)
		rateLimit, rateLimitReset, _ = j.UpdateRateLimit(ctx, outHeaders, "", "")
		if status == 413 {
			rates++
			continue
		}
		// Too many requests
		if status == 429 {
			j.SleepAsRequested(res, thrN)
			sleeps++
			continue
		}
		if err != nil {
			return
		}
		if sleeps > 0 || rates > 0 {
			shared.Printf("recovered after %d sleeps and %d rate limits\n", sleeps, rates)
		}
		break
	}
	data, _ := res.(map[string]interface{})
	fTotal, _ := data["total"].(float64)
	total = int(fTotal)
	iMessages, _ := data["messages"].([]interface{})
	for _, iMessage := range iMessages {
		messages = append(messages, iMessage.(map[string]interface{}))
	}
	// Printf("MESSAGES: %d, TOTAL: %d, OFFSET: %d\n", len(messages), total, offset)
	outRateLimit, outRateLimitReset, newOffset = rateLimit, rateLimitReset, offset+len(messages)
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
	var (
		dateFrom  time.Time
		sDateFrom string
		dateTo    time.Time
		sDateTo   string
	)
	if ctx.DateFrom != nil {
		dateFrom = *ctx.DateFrom
	} else {
		dateFrom = shared.DefaultDateFrom
	}
	sDateFrom = shared.ToESDate(dateFrom)
	if ctx.DateTo != nil {
		dateTo = *ctx.DateTo
	} else {
		dateTo = shared.DefaultDateTo
	}
	sDateTo = shared.ToESDate(dateTo)
	rateLimit, rateLimitReset := -1, -1
	cacheDur := time.Duration(48) * time.Hour
	url := j.URL + "/api/v1/channels.info?roomName=" + neturl.QueryEscape(j.Channel)
	method := "GET"
	headers := map[string]string{"X-User-ID": j.User, "X-Auth-Token": j.Token}
	var (
		res        interface{}
		status     int
		outHeaders map[string][]string
	)
	sleeps, rates := 0, 0
	for {
		err = j.SleepForRateLimit(ctx, rateLimit, rateLimitReset, j.MinRate, j.WaitRate)
		if err != nil {
			return
		}
		// curl -s -H 'X-Auth-Token: token' -H 'X-User-ID: user' URL/api/v1/channels.info?roomName=channel | jq '.'
		// 48 hours for caching channel info
		res, status, _, outHeaders, err = shared.Request(
			ctx,
			url,
			method,
			headers,
			nil,
			nil,
			map[[2]int]struct{}{{200, 200}: {}, {429, 429}: {}}, // JSON statuses: 200, 429
			nil, // Error statuses
			map[[2]int]struct{}{{200, 200}: {}, {429, 429}: {}}, // OK statuses: 200, 429
			map[[2]int]struct{}{{200, 200}: {}},                 // Cache statuses: 200
			true,                                                // retry
			&cacheDur,                                           // cache duration
			false,                                               // skip in dry-run mode
		)
		rateLimit, rateLimitReset, _ = j.UpdateRateLimit(ctx, outHeaders, "", "")
		// Rate limit
		if status == 413 {
			rates++
			continue
		}
		// Too many requests
		if status == 429 {
			sleeps++
			j.SleepAsRequested(res, thrN)
			continue
		}
		if sleeps > 0 || rates > 0 {
			shared.Printf("recovered after %d sleeps and %d rate limits\n", sleeps, rates)
		}
		if err != nil {
			return
		}
		break
	}
	channelInfo, ok := res.(map[string]interface{})["channel"]
	if !ok {
		data, _ := res.(map[string]interface{})
		err = fmt.Errorf("cannot read channel info from:\n%s", data)
		return
	}
	// Process messages (possibly in threads)
	var (
		ch         chan error
		allDocs    []interface{}
		allMsgs    []interface{}
		allMsgsMtx *sync.Mutex
		escha      []chan error
		eschaMtx   *sync.Mutex
	)
	if thrN > 1 {
		ch = make(chan error)
		allMsgsMtx = &sync.Mutex{}
		eschaMtx = &sync.Mutex{}
	}
	nThreads := 0
	processMsg := func(c chan error, item map[string]interface{}) (wch chan error, e error) {
		defer func() {
			if c != nil {
				c <- e
			}
		}()
		esItem := j.AddMetadata(ctx, item)
		if ctx.Project != "" {
			item["project"] = ctx.Project
		}
		esItem["data"] = item
		if allMsgsMtx != nil {
			allMsgsMtx.Lock()
		}
		allMsgs = append(allMsgs, esItem)
		nMsgs := len(allMsgs)
		if nMsgs >= ctx.PackSize {
			sendToQueue := func(c chan error) (ee error) {
				defer func() {
					if c != nil {
						c <- ee
					}
				}()
				// ee = SendToQueue(ctx, j, true, UUID, allMsgs)
				ee = j.RocketchatEnrichItems(ctx, thrN, allMsgs, &allDocs, false)
				if ee != nil {
					shared.Printf("error %v sending %d messages to queue\n", ee, len(allMsgs))
				}
				allMsgs = []interface{}{}
				if allMsgsMtx != nil {
					allMsgsMtx.Unlock()
				}
				return
			}
			if thrN > 1 {
				wch = make(chan error)
				go func() {
					_ = sendToQueue(wch)
				}()
			} else {
				e = sendToQueue(nil)
				if e != nil {
					return
				}
			}
		} else {
			if allMsgsMtx != nil {
				allMsgsMtx.Unlock()
			}
		}
		return
	}
	offset, total := 0, 0
	if thrN > 1 {
		for {
			var messages []map[string]interface{}
			messages, offset, total, rateLimit, rateLimitReset, err = j.GetRocketchatMessages(ctx, sDateFrom, sDateTo, offset, rateLimit, rateLimitReset, thrN)
			if err != nil {
				return
			}
			for _, message := range messages {
				message["channel_info"] = channelInfo
				go func(message map[string]interface{}) {
					var (
						e    error
						esch chan error
					)
					esch, e = processMsg(ch, message)
					if e != nil {
						shared.Printf("process error: %v\n", e)
						return
					}
					if esch != nil {
						if eschaMtx != nil {
							eschaMtx.Lock()
						}
						escha = append(escha, esch)
						if eschaMtx != nil {
							eschaMtx.Unlock()
						}
					}
				}(message)
				nThreads++
				if nThreads == thrN {
					err = <-ch
					if err != nil {
						return
					}
					nThreads--
				}
			}
			if offset >= total {
				break
			}
		}
		for nThreads > 0 {
			err = <-ch
			nThreads--
			if err != nil {
				return
			}
		}
	} else {
		for {
			var messages []map[string]interface{}
			messages, offset, total, rateLimit, rateLimitReset, err = j.GetRocketchatMessages(ctx, sDateFrom, sDateTo, offset, rateLimit, rateLimitReset, thrN)
			if err != nil {
				return
			}
			for _, message := range messages {
				message["channel_info"] = channelInfo
				_, err = processMsg(nil, message)
				if err != nil {
					return
				}
			}
			if offset >= total {
				break
			}
		}
	}
	for _, esch := range escha {
		err = <-esch
		if err != nil {
			return
		}
	}
	nMsgs := len(allMsgs)
	if ctx.Debug > 0 {
		shared.Printf("%d remaining messages to send to queue\n", nMsgs)
	}
	// NOTE: for all items, even if 0 - to flush the queue
	err = j.RocketchatEnrichItems(ctx, thrN, allMsgs, &allDocs, true)
	// err = SendToQueue(ctx, j, true, UUID, allMsgs)
	if err != nil {
		shared.Printf("Error %v sending %d messages to queue\n", err, len(allMsgs))
	}
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
