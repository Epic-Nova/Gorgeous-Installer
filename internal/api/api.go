package api

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

var (
	BaseURL     = "https://api.gorgeous.simsalabim.studio/api/v1"
	IsDevMode   = false
	IsOffline   = false
	httpClient  = &http.Client{Timeout: 10 * time.Second}
)

func init() {
	// Probe for dev mode fallback
	resp, err := http.Get("https://api.gorgeous.simsalabim.studio/")
	if err != nil || resp.TLS == nil {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		// Try HTTP
		resp, err = http.Get("http://api.gorgeous.simsalabim.studio/")
		if err == nil {
			if resp.Body != nil {
				resp.Body.Close()
			}
			BaseURL = "http://api.gorgeous.simsalabim.studio/api/v1"
			IsDevMode = true
		} else {
			IsOffline = true
		}
	} else {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
	}
}

// GenerateSoftLicenseHash computes md5(BaseHash + Salt)
func GenerateSoftLicenseHash(baseHash, salt string) string {
	hasher := md5.New()
	hasher.Write([]byte(baseHash + salt))
	return hex.EncodeToString(hasher.Sum(nil))
}

type InstallerUpdateResponse struct {
	UpdateAvailable bool   `json:"UpdateAvailable"`
	LatestVersion   string `json:"LatestVersion"`
	DownloadUrl     string `json:"DownloadUrl"`
	ReleaseNotes    string `json:"ReleaseNotes"`
	ChecksumSha256  string `json:"ChecksumSha256"`
}

func CheckInstallerUpdate(updateType string) (*InstallerUpdateResponse, error) {
	resp, err := httpClient.Get(BaseURL + "/installer/update-check?type=" + updateType)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var updateResp InstallerUpdateResponse
	if err := json.NewDecoder(resp.Body).Decode(&updateResp); err != nil {
		return nil, err
	}

	return &updateResp, nil
}

type VerifyLicenseRequest struct {
	SystemId       string `json:"SystemId"`
	ModuleCoreHash string `json:"ModuleCoreHash"`
	Salt           string `json:"Salt"`
}

type VerifyLicenseResponse struct {
	IsValid        bool   `json:"IsValid"`
	MatchedVersion string `json:"MatchedVersion"`
	Message        string `json:"Message"`
}

func VerifyLicense(systemId, moduleCoreHash, salt string) (*VerifyLicenseResponse, error) {
	reqBody := VerifyLicenseRequest{
		SystemId:       systemId,
		ModuleCoreHash: moduleCoreHash,
		Salt:           salt,
	}
	jsonData, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", BaseURL+"/auth/verify-license", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("verify-license failed with status %d", resp.StatusCode)
	}

	var verResp VerifyLicenseResponse
	if err := json.NewDecoder(resp.Body).Decode(&verResp); err != nil {
		return nil, err
	}

	return &verResp, nil
}

type PublishRequest struct {
	Version   string `json:"version"`
	Changelog string `json:"changelog"`
	Signature string `json:"signature"`
	Checksum  string `json:"checksum"`
	
	TargetPluginName string `json:"target_plugin_name,omitempty"`
	DisplayName      string `json:"display_name,omitempty"`
	Description      string `json:"description,omitempty"`
	IsCoreSystem     bool   `json:"is_core_system,omitempty"`
}

type PublishResponse struct {
	Success          bool   `json:"Success"`
	SystemId         string `json:"SystemId"`
	PublishedVersion string `json:"PublishedVersion"`
	UploadUrl        string `json:"UploadUrl"` // the pre-signed S3 url
	Message          string `json:"Message"`
}

type ChallengeResponse struct {
	Success   bool   `json:"Success"`
	Error     string `json:"Error"`
	Challenge string `json:"Challenge"`
	ExpiresIn int    `json:"ExpiresIn"`
}

var ErrSystemNotFound = errors.New("SystemNotFound")

func GetPublishChallenge(systemId string) (string, error) {
	req, err := http.NewRequest("GET", BaseURL+"/systems/"+systemId+"/challenge", nil)
	if err != nil {
		return "", err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var challengeResp ChallengeResponse
	if err := json.NewDecoder(resp.Body).Decode(&challengeResp); err != nil {
		return "", err
	}

	if resp.StatusCode == 404 && challengeResp.Error == "SystemNotFound" {
		return challengeResp.Challenge, ErrSystemNotFound
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("failed to fetch challenge, status code %d", resp.StatusCode)
	}

	if !challengeResp.Success {
		return "", fmt.Errorf("API rejected challenge request")
	}

	return challengeResp.Challenge, nil
}

type SystemRegistrationData struct {
	TargetPluginName string
	DisplayName      string
	Description      string
	IsCoreSystem     bool
}

func PublishSystem(systemId, version, changelog, signature, checksum, payloadPath string, regData *SystemRegistrationData) error {
	// 1. Post metadata to get Upload URL
	reqBody := PublishRequest{
		Version:   version,
		Changelog: changelog,
		Signature: signature,
		Checksum:  checksum,
	}
	
	if regData != nil {
		reqBody.TargetPluginName = regData.TargetPluginName
		reqBody.DisplayName = regData.DisplayName
		reqBody.Description = regData.Description
		reqBody.IsCoreSystem = regData.IsCoreSystem
	}
	jsonData, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/systems/%s/publish", BaseURL, systemId), bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API error: status code %d", resp.StatusCode)
	}

	var pubResp PublishResponse
	if err := json.NewDecoder(resp.Body).Decode(&pubResp); err != nil {
		return err
	}

	if !pubResp.Success || pubResp.UploadUrl == "" {
		return errors.New("failed to retrieve S3 upload URL")
	}

	// 2. Upload zip payload directly to S3
	file, err := os.Open(payloadPath)
	if err != nil {
		return err
	}
	defer file.Close()

	fileStat, _ := file.Stat()
	putReq, err := http.NewRequest("PUT", pubResp.UploadUrl, file)
	if err != nil {
		return err
	}
	putReq.ContentLength = fileStat.Size()
	putReq.Header.Set("Content-Type", "application/zip")

	s3Client := &http.Client{Timeout: 5 * time.Minute}
	s3Resp, err := s3Client.Do(putReq)
	if err != nil {
		return err
	}
	defer s3Resp.Body.Close()

	if s3Resp.StatusCode >= 400 {
		body, _ := io.ReadAll(s3Resp.Body)
		return fmt.Errorf("S3 upload failed: %d - %s", s3Resp.StatusCode, string(body))
	}

	return nil
}

type SystemItem struct {
	SystemId         string   `json:"SystemId"`
	TargetPluginName string   `json:"TargetPluginName"`
	DisplayName      string   `json:"DisplayName"`
	Description      string   `json:"Description"`
	Version          string   `json:"Version"`
	DownloadUrl      string   `json:"DownloadUrl"`
	SourcePaths      []string `json:"SourcePaths"`
	ContentPaths     []string `json:"ContentPaths"`
	IsCoreSystem     bool     `json:"bIsCoreSystem"`
}

type SystemsResponse struct {
	OfflineSystemCache []SystemItem `json:"OfflineSystemCache"`
}

func GetSystems() ([]SystemItem, error) {
	req, err := http.NewRequest("GET", BaseURL+"/systems", nil)
	if err != nil {
		return nil, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error: status code %d", resp.StatusCode)
	}

	var sysResp SystemsResponse
	if err := json.NewDecoder(resp.Body).Decode(&sysResp); err != nil {
		return nil, err
	}

	return sysResp.OfflineSystemCache, nil
}

func PatchSystem(systemId string, signature string, regData SystemRegistrationData) error {
	reqBody := struct {
		Signature        string `json:"signature"`
		TargetPluginName string `json:"target_plugin_name"`
		DisplayName      string `json:"display_name"`
		Description      string `json:"description"`
		IsCoreSystem     bool   `json:"is_core_system"`
	}{
		Signature:        signature,
		TargetPluginName: regData.TargetPluginName,
		DisplayName:      regData.DisplayName,
		Description:      regData.Description,
		IsCoreSystem:     regData.IsCoreSystem,
	}

	jsonData, _ := json.Marshal(reqBody)
	req, err := http.NewRequest("PATCH", BaseURL+"/systems/"+systemId, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("failed to patch system, status code %d", resp.StatusCode)
	}
	return nil
}

func DeleteSystem(systemId string, signature string) error {
	reqBody := struct {
		Signature string `json:"signature"`
	}{
		Signature: signature,
	}

	jsonData, _ := json.Marshal(reqBody)
	req, err := http.NewRequest("DELETE", BaseURL+"/systems/"+systemId, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("failed to delete system, status code %d", resp.StatusCode)
	}
	return nil
}
