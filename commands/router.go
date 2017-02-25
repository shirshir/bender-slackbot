package commands

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

// ResetMediaServer is now commented
func ResetMediaServer() bool {
	result := false

	if runningFritzboxTunnel() {
		tlsConfig := &tls.Config{InsecureSkipVerify: true}
		transport := &http.Transport{TLSClientConfig: tlsConfig}
		client := &http.Client{Transport: transport}

		// Generated by curl-to-Go: https://mholt.github.io/curl-to-go

		// Login to Router
		body := strings.NewReader(`group_id=&action_mode=&action_script=&action_wait=5&current_page=Main_Login.asp&next_page=index.asp&login_authorization=YWRtaW46UnVtcDEzU3RpMXoh`)
		req, err := http.NewRequest("POST", "https://192.168.1.1:8443/login.cgi", body)
		if err != nil {
			fmt.Println(err)
		}
		resp, err2 := client.Do(req)
		if err2 != nil {
			fmt.Println(err2)
		}
		defer resp.Body.Close()
		// get Cookie variable value from [asus_token=xyz123; HttpOnly;]
		asusToken := strings.TrimLeft(strings.TrimRight(resp.Cookies()[0].Raw, " HttpOnly;"), "=")

		// Restart Media Server
		body2 := strings.NewReader(`preferred_lang=EN&firmver=3.0.0.4&current_page=mediaserver.asp&next_page=mediaserver.asp&flag=nodetect&action_mode=apply&action_script=restart_media&action_wait=5&daapd_enable=0&dms_enable=1&dms_dir_x=%3C%2Fmnt%2FTOSHIBA_EXT%2FDLNA&dms_dir_type_x=%3CV&dms_dir_manual=1&daapd_friendly_name=RT-AC88U-D7F8&dms_friendly_name=EntertainME&dms_rebuild=0&dms_web=1&dms_dir_manual_x=1&type_A_audio=on&type_P_image=on&type_V_video=on`)
		req2, err3 := http.NewRequest("POST", "https://192.168.1.1:8443/start_apply.htm", body2)
		if err3 != nil {
			fmt.Println(err3)
		}
		req2.Header.Set("Cookie", asusToken)
		req2.Header.Set("Referer", "https://192.168.1.1:8443/mediaserver.asp")
		req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp2, err4 := client.Do(req2)
		if err4 != nil {
			fmt.Println(err4)
		}
		defer resp2.Body.Close()
		if resp.StatusCode == 200 { // OK
			bodyBytes, _ := ioutil.ReadAll(resp2.Body)
			bodyString := string(bodyBytes)
			if strings.Contains(bodyString, "no_changes_and_no_committing()") {
				result = true
			}
		}
	}

	return result
}