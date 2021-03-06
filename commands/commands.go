package commands

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/scrypt"

	"github.com/danackerson/digitalocean/common"
	"github.com/elgs/gojq"
	"github.com/nlopes/slack"
)

var raspberryPIIP = os.Getenv("raspberryPIIP")
var rtm *slack.RTM
var piSDCardPath = "/home/pi/torrents/"
var piUSBMountPoint = "/mnt/usb_1"
var piUSBMountPath = piUSBMountPoint + "/DLNA/torrents/"
var routerIP = "192.168.1.1"
var tranc = "tranc"

var circleCIDoAlgoURL = "https://circleci.com/api/v1.1/project/github/danackerson/do-algo"
var circleCITokenParam = "?circle-token=" + os.Getenv("circleAPIToken")

// SlackReportChannel default reporting channel for bot crons
var SlackReportChannel = os.Getenv("slackReportChannel") // C33QYV3PW is #remote_network_report

// SetRTM sets singleton
func SetRTM(rtmPassed *slack.RTM) {
	rtm = rtmPassed
}

// CheckCommand is now commented
func CheckCommand(api *slack.Client, slackMessage slack.Msg, command string) {
	args := strings.Fields(command)
	callingUserProfile, _ := api.GetUserInfo(slackMessage.User)
	params := slack.PostMessageParameters{AsUser: true}

	if args[0] == "yt" {
		if len(args) > 1 {
			// strip '<>' off url
			downloadURL := strings.Trim(args[1], "<>")
			uri, err := url.ParseRequestURI(downloadURL)
			if err != nil {
				api.PostMessage(slackMessage.Channel, "Invalid URL for downloading! ("+err.Error()+")", params)
			} else {
				downloadYoutubeVideo(uri.String())
				api.PostMessage(slackMessage.Channel, "Requested YouTube video...", params)
			}
		} else {
			api.PostMessage(slackMessage.Channel, "Please provide YouTube video URL!", params)
		}
	} else if args[0] == "bb" {
		// TODO pass yesterday's date
		response := ShowBaseBallGames()
		result := "Ball games from " + response.ReadableDate + ":\n"

		for _, gameMetaData := range response.Games {
			watchURL := "<" + gameMetaData[10] + "|" + gameMetaData[0] + " @ " + gameMetaData[4] + ">    "
			downloadURL := "<https://ackerson.de/bb_download?fileType=bb&gameTitle=" + gameMetaData[2] + "-" + gameMetaData[6] + "__" + response.ReadableDate + "&gameURL=" + gameMetaData[10] + " | :smartphone:>"

			result += watchURL + downloadURL + "\n"
		}

		api.PostMessage(slackMessage.Channel, result, params)
	} else if args[0] == "algo" {
		response := ListDODroplets(true)

		if strings.Contains(response, "york.shire") {
			response = findAndReturnVPNConfigs(response)
			api.PostMessage(slackMessage.Channel, response, params)
		} else {
			building, buildNum, _ := circleCIDoAlgoBuildingAndBuildNums()
			if !building {
				buildsURL := circleCIDoAlgoURL + circleCITokenParam
				buildsParser := getJSONFromRequestURL(buildsURL, "POST")

				buildNumParse, _ := buildsParser.Query("build_num")
				buildNum = strconv.FormatFloat(buildNumParse.(float64), 'f', -1, 64)
			}
			response = ":circleci: <https://circleci.com/gh/danackerson/do-algo/" + buildNum + "|do-algo Build " + buildNum + ">"
			api.PostMessage(slackMessage.Channel, response, params)
		}
	} else if args[0] == "do" {
		response := ListDODroplets(true)
		api.PostMessage(slackMessage.Channel, response, params)
	} else if args[0] == "dd" {
		if len(args) > 1 {
			number, err := strconv.Atoi(args[1])
			if err != nil {
				api.PostMessage(slackMessage.Channel, "Invalid integer value for ID!", params)
			} else {
				result := common.DeleteDODroplet(number)
				api.PostMessage(slackMessage.Channel, result, params)
			}
		} else {
			api.PostMessage(slackMessage.Channel, "Please provide Droplet ID from `do` cmd!", params)
		}
	} else if args[0] == "fsck" {
		if runningFritzboxTunnel() {
			response := ""

			if len(args) > 1 {
				path := strings.Join(args[1:], " ")
				response += CheckPiDiskSpace(path)
			} else {
				response += CheckPiDiskSpace("")
			}

			rtm.SendMessage(rtm.NewOutgoingMessage(response, slackMessage.Channel))
		}
	} else if args[0] == "mv" || args[0] == "rm" {
		response := ""
		if len(args) > 1 {
			if runningFritzboxTunnel() {
				path := strings.Join(args[1:], " ")
				if args[0] == "rm" {
					response = DeleteTorrentFile(path)
				} else {
					MoveTorrentFile(api, path)
				}

				rtm.SendMessage(rtm.NewOutgoingMessage(response, slackMessage.Channel))
			}
		} else {
			rtm.SendMessage(rtm.NewOutgoingMessage("Please provide a filename", slackMessage.Channel))
		}
	} else if args[0] == "torq" {
		var response string
		cat := 0
		if len(args) > 1 {
			if args[1] == "nfl" {
				cat = 200
			} else if args[1] == "ubuntu" {
				cat = 300
			}

			searchString := strings.Join(args, " ")
			searchString = strings.TrimPrefix(searchString, "torq ")
			_, response = SearchFor(searchString, Category(cat))
		} else {
			_, response = SearchFor("", Category(cat))
		}
		api.PostMessage(slackMessage.Channel, response, params)
	} else if args[0] == "ovpn" {
		response := RaspberryPIPrivateTunnelChecks(true)
		rtm.SendMessage(rtm.NewOutgoingMessage(response, slackMessage.Channel))
	} else if args[0] == "sw" {
		response := ":partly_sunny_rain: <https://www.wunderground.com/cgi-bin/findweather/getForecast?query=" +
			"48.3,11.35#forecast-graph|10-day forecast Schwabhausen>"
		api.PostMessage(slackMessage.Channel, response, params)
	} else if args[0] == "vpnc" {
		result := vpnTunnelCmds("/usr/sbin/vpnc-connect", "fritzbox")
		rtm.SendMessage(rtm.NewOutgoingMessage(result, slackMessage.Channel))
	} else if args[0] == "vpnd" {
		result := vpnTunnelCmds("/usr/sbin/vpnc-disconnect")
		rtm.SendMessage(rtm.NewOutgoingMessage(result, slackMessage.Channel))
	} else if args[0] == "vpns" {
		result := vpnTunnelCmds("status")
		rtm.SendMessage(rtm.NewOutgoingMessage(result, slackMessage.Channel))
	} else if args[0] == "trans" || args[0] == "trand" || args[0] == tranc {
		if runningFritzboxTunnel() {
			response := torrentCommand(args)
			api.PostMessage(slackMessage.Channel, response, params)
		}
	} else if args[0] == "mvv" {
		response := "<https://img.srv2.de/customer/sbahnMuenchen/newsticker/newsticker.html|Aktuelles>"
		response += " | <" + mvvRoute("Schwabhausen", "München, Hauptbahnhof") + "|Going in>"
		response += " | <" + mvvRoute("München, Hauptbahnhof", "Schwabhausen") + "|Going home>"

		api.PostMessage(slackMessage.Channel, response, params)
	} else if args[0] == "help" {
		response := ":sun_behind_rain_cloud: `sw`: Schwabhausen weather\n" +
			":metro: `mvv`: Status | Trip In | Trip Home\n" +
			":closed_lock_with_key: `vpn[c|s|d]`: [C]onnect, [S]tatus, [D]rop VPN tunnel to Fritz!Box\n" +
			":openvpn: `ovpn`: show status of OVPN.se on :raspberry_pi:\n" +
			":algovpn: `algo`: show|launch AlgoVPN droplet on :do_droplet:\n" +
			":do_droplet: `do|dd <id>`: show|delete DigitalOcean droplet(s)\n" +
			":pirate_bay: `torq <search term>`\n" +
			":transmission: `tran[c|s|d]`: [C]reate <URL>, [S]tatus, [D]elete <ID> torrents on :raspberry_pi:\n" +
			":recycle: `rm(|mv) <filename>` from :raspberry_pi: (to `" + piUSBMountPath + "`)\n" +
			":floppy_disk: `fsck`: show disk space on :raspberry_pi:\n" +
			":baseball: `bb`: show yesterday's baseball games\n" +
			":youtube: `yt <video url>`: Download Youtube video to Papa's handy\n"
		api.PostMessage(slackMessage.Channel, response, params)
	} else {
		rtm.SendMessage(rtm.NewOutgoingMessage("whaddya say <@"+callingUserProfile.Name+">? Try `help` instead...",
			slackMessage.Channel))
	}
}

func circleCIDoAlgoBuildingAndBuildNums() (bool, string, string) {
	lastSuccessBuildNum := "-1"
	currentBuildNum := "-1"
	currentlyBuilding := true

	buildsURL := circleCIDoAlgoURL + circleCITokenParam
	buildsParser := getJSONFromRequestURL(buildsURL, "GET")
	array, _ := buildsParser.QueryToArray(".")
	for i := 0; i < len(array); i++ {
		statusStr, _ := buildsParser.Query("[" + strconv.Itoa(i) + "].status")

		if i == 0 {
			log.Println("current status: " + statusStr.(string))
			currentlyBuilding = !isFinishedStatus(statusStr.(string))
			buildNumParse, _ := buildsParser.Query("[" + strconv.Itoa(i) + "].build_num")
			currentBuildNum = strconv.FormatFloat(buildNumParse.(float64), 'f', -1, 64)
		}

		if statusStr.(string) == "success" || statusStr.(string) == "fixed" {
			buildNumParse, _ := buildsParser.Query("[" + strconv.Itoa(i) + "].build_num")
			lastSuccessBuildNum = strconv.FormatFloat(buildNumParse.(float64), 'f', -1, 64)
			break
		}
	}

	return currentlyBuilding, currentBuildNum, lastSuccessBuildNum
}

func isFinishedStatus(status string) bool {
	switch status {
	case
		"canceled",
		"success",
		"fixed",
		"failed":
		return true
	}
	return false
}

func getJSONFromRequestURL(url string, requestType string) *gojq.JQ {
	req, _ := http.NewRequest(requestType, url, nil)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println("failed to call "+url+": ", err)
	} else {
		log.Println("successfully called: " + url)
	}
	defer resp.Body.Close()

	contentBytes, _ := ioutil.ReadAll(resp.Body)
	contentParser, _ := gojq.NewStringQuery(string(contentBytes))

	return contentParser
}

func waitAndRetrieveLogs(buildURL string, index int) string {
	outputURL := "N/A"

	for notReady := true; notReady; notReady = (outputURL == "N/A") {
		buildParser := getJSONFromRequestURL(buildURL, "GET")
		actionsParser, errOutput := buildParser.Query("steps.[" + strconv.Itoa(index) + "].actions.[0].output_url")

		if errOutput != nil {
			log.Println("waitAndRetrieveLogs: " + errOutput.Error())
			time.Sleep(5000 * time.Millisecond)
		} else {
			outputURL = actionsParser.(string)
		}
	}

	return outputURL
}

func findAndReturnVPNConfigs(doServers string) string {
	passAlgoVPN := "No successful AlgoVPN deployments found."
	links := ""

	building, _, lastSuccessBuildNum := circleCIDoAlgoBuildingAndBuildNums()

	if !building && lastSuccessBuildNum != "-1" {
		// now get build details for this buildNum
		var outputURL string
		buildURL := circleCIDoAlgoURL + "/" + lastSuccessBuildNum + circleCITokenParam
		buildParser := getJSONFromRequestURL(buildURL, "GET")
		for i := 0; i < 9; i++ {
			stepName, _ := buildParser.Query("steps.[" + strconv.Itoa(i) + "].name")
			if stepName == "Upload to DockerHub, deploy to Digital Ocean Droplet & launch VPN" {
				outputURL = waitAndRetrieveLogs(buildURL, i)
				break
			}
		}

		// get the log output for this step and parse out IP address and SSH password
		outputParser := getJSONFromRequestURL(outputURL, "GET")
		message, _ := outputParser.Query("[0].message")
		//msgs := strings.Split(message.(string), "\n")

		checkPassString, _ := regexp.Compile(`The p12 and SSH keys password for new users is (?:[0-9a-f]{8})`)
		passAlgoVPN = string(checkPassString.Find([]byte(message.(string))))

		ipv4 := getIPv4Address(doServers)
		log.Println(ipv4)

		// lets encrypt the filenames on disk
		doPersonalAccessToken := os.Getenv("digitalOceanToken")
		salt := []byte(ipv4 + ":" + doPersonalAccessToken)
		desktopConfigFileHashed, _ := scrypt.Key([]byte("dan.mobileconfig"), salt, 16384, 8, 1, 32)
		desktopConfigFileString := hex.EncodeToString(desktopConfigFileHashed)
		fmt.Println(desktopConfigFileString)

		mobileConfigFileHashed, _ := scrypt.Key([]byte("android_dan.sswan"), salt, 16384, 8, 1, 32)
		mobileConfigFileString := hex.EncodeToString(mobileConfigFileHashed)
		fmt.Println(mobileConfigFileString)

		copyCmd := "cp /algo_vpn/" + ipv4 + "/dan.mobileconfig /uploads/" + desktopConfigFileString + " && cp /algo_vpn/" + ipv4 + "/android_dan.sswan /uploads/" + mobileConfigFileString
		_, err := exec.Command("/bin/bash", "-c", copyCmd).Output()
		if err != nil {
			fmt.Printf("Failed to execute command: %s", copyCmd)
		}

		joinStatus := "*Import* VPN profile"
		resp, _ := http.Get("https://ackerson.de/bb_download?fileType=vpn&gameTitle=android_dan.sswan&gameURL=" + mobileConfigFileString)
		if resp.StatusCode != 200 {
			joinStatus = "couldn't send to Papa's handy"
		}

		links = ":link: <https://ackerson.de/bb_games/" + mobileConfigFileString + "|android_dan_" + ipv4 + ".sswan> (" + joinStatus + ")\n"
		links += ":link: <https://ackerson.de/bb_games/" + desktopConfigFileString + "|dan.mobileconfig> (dbl click on Mac)\n"
	}

	return ":algovpn: " + passAlgoVPN + "\n" + links
}

func getIPv4Address(serverList string) string {
	var ipV4 []byte

	parts := strings.Split(serverList, "\n")
	for i := range parts {
		// FORMAT => ":do_droplet: <addr|name> (IPv4) [ID: DO_ID]"
		if strings.Contains(parts[i], "york.shire") {
			reIPv4, _ := regexp.Compile(`(?:[0-9]{1,3}\.){3}[0-9]{1,3}`)
			ipV4 = reIPv4.Find([]byte(parts[i]))
			break
		}
	}

	return string(ipV4)
}

func mvvRoute(origin string, destination string) string {
	loc, _ := time.LoadLocation("Europe/Berlin")
	date := time.Now().In(loc)

	yearObj := date.Year()
	monthObj := int(date.Month())
	dayObj := date.Day()
	hourObj := date.Hour()
	minuteObj := date.Minute()

	month := strconv.Itoa(monthObj)
	hour := strconv.Itoa(hourObj)
	day := strconv.Itoa(dayObj)
	minute := strconv.Itoa(minuteObj)
	year := strconv.Itoa(yearObj)

	return "http://efa.mvv-muenchen.de/mvv/XSLT_TRIP_REQUEST2?&language=de" +
		"&anyObjFilter_origin=0&sessionID=0&itdTripDateTimeDepArr=dep&type_destination=any" +
		"&itdDateMonth=" + month + "&itdTimeHour=" + hour + "&anySigWhenPerfectNoOtherMatches=1" +
		"&locationServerActive=1&name_origin=" + origin + "&itdDateDay=" + day + "&type_origin=any" +
		"&name_destination=" + destination + "&itdTimeMinute=" + minute + "&Session=0&stateless=1" +
		"&SpEncId=0&itdDateYear=" + year
}

func downloadYoutubeVideo(origURL string) bool {
	resp, _ := http.Get("https://ackerson.de/bb_download?gameURL=" + origURL)
	if resp.StatusCode == 200 {
		return true
	}

	return false
}
