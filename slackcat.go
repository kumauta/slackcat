package main

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/bluele/slack"
)

var (
	msgOpts = &slack.ChatPostMessageOpt{AsUser: true}
)

//Slackcat client
type Slackcat struct {
	api         *slack.Slack
	queue       *StreamQ
	shutdown    chan os.Signal
	channelID   string
	channelName string
}

func newSlackcat(token, channelName string) *Slackcat {
	sc := &Slackcat{
		api:         slack.New(token),
		queue:       newStreamQ(),
		shutdown:    make(chan os.Signal, 1),
		channelName: channelName,
	}

	res, err := sc.api.AuthTest()
	failOnError(err, "Slack API Error")
	output(fmt.Sprintf("connected to %s as %s", res.Team, res.User))
	sc.channelID = sc.lookupSlackID()

	signal.Notify(sc.shutdown, os.Interrupt)
	return sc
}

//Lookup Slack id for channel, group, or im by name
func (sc *Slackcat) lookupSlackID() string {
	api := sc.api
	if channel, err := api.FindChannelByName(sc.channelName); err == nil {
		return channel.Id
	}
	if group, err := api.FindGroupByName(sc.channelName); err == nil {
		return group.Id
	}
	if im, err := api.FindImByName(sc.channelName); err == nil {
		return im.Id
	}
	exitErr(fmt.Errorf("No such channel, group, or im"))
	return ""
}

func (sc *Slackcat) trap() {
	sigcount := 0
	for sig := range sc.shutdown {
		if sigcount > 0 {
			exitErr(fmt.Errorf("aborted"))
		}
		output(fmt.Sprintf("got signal: %s", sig.String()))
		output("press ctrl+c again to exit immediately")
		sigcount++
		go sc.exit()
	}
}

func (sc *Slackcat) exit() {
	for {
		if sc.queue.IsEmpty() {
			os.Exit(0)
		} else {
			output("flushing remaining messages to Slack...")
			time.Sleep(3 * time.Second)
		}
	}
}

func (sc *Slackcat) stream(lines chan string) {
	output("starting stream")

	go func() {
		for line := range lines {
			sc.queue.Add(line)
		}
		sc.exit()
	}()

	go sc.processStreamQ()
	go sc.trap()
	select {}
}

func (sc *Slackcat) processStreamQ() {
	if !(sc.queue.IsEmpty()) {
		msglines := sc.queue.Flush()
		if noop {
			output(fmt.Sprintf("skipped posting of %s message lines to %s", strconv.Itoa(len(msglines)), sc.channelName))
		} else {
			cnt := 1
			bs := []string{"```"}
			postMsg := []string{}
			for i := 0; i < len(msglines); i++ {

				postMsg = append(postMsg,msglines[i:i+1]...)
				if 30 == len(postMsg) {
					tmp := append(bs, postMsg[0:30]...)
					tmp = append(tmp, bs...)
					sc.postMsg(tmp)
					postMsg = []string{}
				}else if cnt == len(msglines) {
					tmp := append(bs, postMsg[0:len(postMsg)]...)
					tmp = append(tmp, bs...)
					sc.postMsg(tmp)
					postMsg = []string{}
				}
				cnt ++
			}
		}
		sc.queue.Ack()
	}
	time.Sleep(3 * time.Second)
	sc.processStreamQ()
}

func (sc *Slackcat) postMsg(msglines []string) {
	msg := strings.Join(msglines, "\n")
	msg = strings.Replace(msg, "&", "%26amp%3B", -1)
	msg = strings.Replace(msg, "<", "%26lt%3B", -1)
	msg = strings.Replace(msg, ">", "%26gt%3B", -1)

	err := sc.api.ChatPostMessage(sc.channelID, msg, msgOpts)
	failOnError(err)
	count := strconv.Itoa(len(msglines))
	output(fmt.Sprintf("posted %s message lines to %s", count, sc.channelName))
}

func (sc *Slackcat) postFile(filePath, fileName, fileType, fileComment string) {
	//default to timestamp for filename
	if fileName == "" {
		fileName = strconv.FormatInt(time.Now().Unix(), 10)
	}

	if noop {
		output(fmt.Sprintf("skipping upload of file %s to %s", fileName, sc.channelName))
		return
	}

	start := time.Now()
	err := sc.api.FilesUpload(&slack.FilesUploadOpt{
		Filepath:       filePath,
		Filename:       fileName,
		Filetype:       fileType,
		Title:          fileName,
		InitialComment: fileComment,
		Channels:       []string{sc.channelID},
	})
	failOnError(err, "error uploading file to Slack")
	duration := strconv.FormatFloat(time.Since(start).Seconds(), 'f', 3, 64)
	output(fmt.Sprintf("file %s uploaded to %s (%ss)", fileName, sc.channelName, duration))
	os.Exit(0)
}
