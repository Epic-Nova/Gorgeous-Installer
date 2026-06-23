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
	"strings"
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

func CheckInstallerUpdate(updateType string, currentVersion string) (*InstallerUpdateResponse, error) {
	url := BaseURL + "/installer/update-check?type=" + updateType
	if currentVersion != "" {
		url += "&current_version=" + currentVersion
	}
	resp, err := httpClient.Get(url)
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
	IsCoreSystem     *bool   `json:"is_core_system,omitempty"`
	MinimumCoreVersion string `json:"minimum_core_version,omitempty"`
	SourcePaths      []string `json:"source_paths,omitempty"`
	ContentPaths     []string `json:"content_paths,omitempty"`
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

	if resp.StatusCode == 404 {
		var challengeResp ChallengeResponse
		json.NewDecoder(resp.Body).Decode(&challengeResp)
		return challengeResp.Challenge, ErrSystemNotFound
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("failed to fetch challenge, status code %d", resp.StatusCode)
	}

	var challengeResp ChallengeResponse
	if err := json.NewDecoder(resp.Body).Decode(&challengeResp); err != nil {
		return "", err
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
	MinimumCoreVersion string
	SourcePaths      []string
	ContentPaths     []string
}

func PublishSystem(systemId, version, changelog, signature, checksum, payloadPath, minimumCoreVersion string, regData *SystemRegistrationData, sourcePaths []string, contentPaths []string) error {
	// 1. Post metadata to get Upload URL
	reqBody := PublishRequest{
		Version:            version,
		Changelog:          changelog,
		Signature:          signature,
		Checksum:           checksum,
		MinimumCoreVersion: minimumCoreVersion,
		SourcePaths:        sourcePaths,
		ContentPaths:       contentPaths,
	}
	
	if regData != nil {
		reqBody.TargetPluginName = regData.TargetPluginName
		reqBody.DisplayName = regData.DisplayName
		reqBody.Description = regData.Description
		reqBody.IsCoreSystem = &regData.IsCoreSystem
		if regData.MinimumCoreVersion != "" {
			reqBody.MinimumCoreVersion = regData.MinimumCoreVersion
		}
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

	s3Resp, err := http.DefaultClient.Do(putReq)
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

func PromoteSystemVersion(systemId, version string) error {
	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/systems/%s/versions/%s/promote", BaseURL, systemId, version), nil)
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("promote failed: %d - %s", resp.StatusCode, string(body))
	}
	return nil
}

type SystemVersionItem struct {
	Version        string `json:"Version"`
	IsMasterUpdate bool   `json:"IsMasterUpdate"`
}

type SystemItem struct {
	SystemId           string              `json:"SystemId"`
	TargetPluginName   string              `json:"TargetPluginName"`
	DisplayName        string              `json:"DisplayName"`
	Description        string              `json:"Description"`
	Version            string              `json:"Version"`
	ChangelogUrl       string              `json:"ChangelogUrl"`
	DownloadUrl        string              `json:"DownloadUrl"`
	SourcePaths        []string            `json:"SourcePaths"`
	ContentPaths       []string            `json:"ContentPaths"`
	Versions           []SystemVersionItem `json:"Versions"`
	IsCoreSystem       bool                `json:"bIsCoreSystem"`
	MinimumCoreVersion string              `json:"MinimumCoreVersion"`
}

func (s *SystemItem) IsPackUpdate() bool {
	return len(s.SourcePaths) > 0 && len(s.ContentPaths) > 0
}

func (s *SystemItem) IsInstallerUpdate() bool {
	return strings.HasPrefix(s.SystemId, "GorgeousInstaller")
}

func (s *SystemItem) IsPluginUpdate() bool {
	if strings.HasPrefix(s.SystemId, "GorgeousInstaller") {
		return false
	}
	return len(s.SourcePaths) == 0 && len(s.ContentPaths) == 0
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

func GetAllSystems() ([]SystemItem, error) {
	req, err := http.NewRequest("GET", BaseURL+"/systems/all", nil)
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
		Signature           string   `json:"signature"`
		TargetPluginName    string   `json:"target_plugin_name"`
		DisplayName         string   `json:"display_name"`
		Description         string   `json:"description"`
		MinimumCoreVersion  string   `json:"minimum_core_version"`
		SourcePaths         []string `json:"source_paths"`
		ContentPaths        []string `json:"content_paths"`
		IsCoreSystem        *bool    `json:"is_core_system,omitempty"`
	}{
		Signature:           signature,
		TargetPluginName:    regData.TargetPluginName,
		DisplayName:         regData.DisplayName,
		Description:         regData.Description,
		MinimumCoreVersion:  regData.MinimumCoreVersion,
		SourcePaths:         regData.SourcePaths,
		ContentPaths:        regData.ContentPaths,
	}

	isCore := regData.IsCoreSystem
	reqBody.IsCoreSystem = &isCore

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

func DeleteSystemVersion(systemId string, version string, signature string) error {
	reqBody := struct {
		Signature string `json:"signature"`
	}{Signature: signature}

	jsonData, _ := json.Marshal(reqBody)
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/systems/%s/versions/%s", BaseURL, systemId, version), bytes.NewBuffer(jsonData))
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
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete version failed: %d - %s", resp.StatusCode, string(body))
	}

	return nil
}
