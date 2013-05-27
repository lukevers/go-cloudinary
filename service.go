// Copyright 2013 Mathias Monnerville. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package cloudinary provides support for managing static assets
// on the Cloudinary service.
//
// The Cloudinary service allows image and raw files management in
// the cloud.
package cloudinary

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	baseUploadUrl = "http://api.cloudinary.com/v1_1"
	imageType     = "image"
	rawType       = "raw"
)

type ResourceType int

const (
	ImageType ResourceType = iota
	RawType
)

type Service struct {
	cloudName     string
	apiKey        string
	apiSecret     string
	uploadURI     *url.URL     // To upload resources
	adminURI      *url.URL     // To use the admin API
	mongoDbURI    *url.URL     // Can be nil: upload sync disabled
	uploadResType ResourceType // Upload resource type
}

type Image struct {
	PublicId     string `json:"public_id"`
	Format       string `json:"format"`
	Version      int    `json:"version"`
	ResourceType string `json:"resource_type"` // image or raw
	Size         int    `json:"bytes"`         // In bytes
	Url          string `json:"url"`           // Remote url
	SecureUrl    string `json:"secure_url"`    // Over https
}

type pagination struct {
	NextCursor int64 `json: "next_cursor"`
}

type imageList struct {
	pagination
	Resources []*Image `json: "resources"`
}

// Upload response after uploading a file.
type uploadResponse struct {
	PublicId     string `json:"public_id"`
	Version      uint   `json:"version"`
	Format       string `json:"format"`
	ResourceType string `json:"resource_type"` // "image" or "raw"
}

// Dial will use the url to connect to the Cloudinary service.
// The uri parameter must be a valid URI with the cloudinary:// scheme,
// e.g.
//  cloudinary://api_key:api_secret@cloud_name
func Dial(uri string) (*Service, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "cloudinary" {
		return nil, errors.New("Missing cloudinary:// scheme in URI")
	}
	secret, exists := u.User.Password()
	if !exists {
		return nil, errors.New("No API secret provided in URI.")
	}
	s := &Service{
		cloudName:     u.Host,
		apiKey:        u.User.Username(),
		apiSecret:     secret,
		uploadResType: ImageType,
	}
	// Default upload URI to the service. Can change at runtime in the
	// Upload() function for raw file uploading.
	up, err := url.Parse(fmt.Sprintf("%s/%s/image/upload/", baseUploadUrl, s.cloudName))
	if err != nil {
		return nil, err
	}
	s.uploadURI = up

	// Admin API url
	adm, err := url.Parse(fmt.Sprintf("%s/%s", baseAdminUrl, s.cloudName))
	if err != nil {
		return nil, err
	}
	adm.User = url.UserPassword(s.apiKey, s.apiSecret)
	s.adminURI = adm
	return s, nil
}

// UseDatabase connects to a mongoDB database and stores upload JSON
// responses, along with a source file checksum to prevent uploading
// the same file twice. Stored information is used by Url() to build
// a public URL for accessing the uploaded resource.
func (s *Service) UseDatabase(mongoDbURI string) error {
	u, err := url.Parse(mongoDbURI)
	if err != nil {
		return err
	}
	if u.Scheme != "mongodb" {
		return errors.New("Missing mongodb:// scheme in URI")
	}
	s.mongoDbURI = u
	return nil
}

// CloudName returns the cloud name used to access the Cloudinary service.
func (s *Service) CloudName() string {
	return s.cloudName
}

// ApiKey returns the API key used to access the Cloudinary service.
func (s *Service) ApiKey() string {
	return s.apiKey
}

// DefaultUploadURI returns the default URI used to upload images to the Cloudinary service.
func (s *Service) DefaultUploadURI() *url.URL {
	return s.uploadURI
}

// cleanAssetName returns an asset name from the parent dirname and
// the file name without extension. The path /tmp/css/default.css will
// return css/default.
func cleanAssetName(path string) string {
	idx := strings.LastIndex(path, string(os.PathSeparator))
	if idx != -1 {
		idx = strings.LastIndex(path[:idx], string(os.PathSeparator))
	}
	publicId := path[idx+1:]
	return publicId[:len(publicId)-len(filepath.Ext(publicId))]
}

func (s *Service) walkIt(path string, info os.FileInfo, err error) error {
	if info.IsDir() {
		return nil
	}
	if err := s.uploadFile(path, false); err != nil {
		return err
	}
	return nil
}

// Upload file to the service. See Upload().
func (s *Service) uploadFile(path string, randomPublicId bool) error {
	buf := new(bytes.Buffer)
	w := multipart.NewWriter(buf)

	// Write public ID
	var publicId string
	if !randomPublicId {
		publicId = cleanAssetName(path)
		pi, err := w.CreateFormField("public_id")
		if err != nil {
			return err
		}
		pi.Write([]byte(publicId))
	}

	// Write API key
	ak, err := w.CreateFormField("api_key")
	if err != nil {
		return err
	}
	ak.Write([]byte(s.apiKey))

	// Write timestamp
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	ts, err := w.CreateFormField("timestamp")
	if err != nil {
		return err
	}
	ts.Write([]byte(timestamp))

	// Write signature
	hash := sha1.New()
	part := fmt.Sprintf("timestamp=%s%s", timestamp, s.apiSecret)
	if !randomPublicId {
		part = fmt.Sprintf("public_id=%s&%s", publicId, part)
	}
	io.WriteString(hash, part)
	signature := fmt.Sprintf("%x", hash.Sum(nil))

	si, err := w.CreateFormField("signature")
	if err != nil {
		return err
	}
	si.Write([]byte(signature))

	// Write file field
	fw, err := w.CreateFormFile("file", path)
	if err != nil {
		return err
	}
	fd, err := os.Open(path)
	if err != nil {
		return err
	}
	defer fd.Close()

	_, err = io.Copy(fw, fd)
	if err != nil {
		return err
	}
	// Don't forget to close the multipart writer to get a terminating boundary
	w.Close()

	upURI := s.uploadURI.String()
	if s.uploadResType == RawType {
		upURI = strings.Replace(upURI, imageType, rawType, 1)
	}
	req, err := http.NewRequest("POST", upURI, buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		return err
	}

	if resp.StatusCode == http.StatusOK {
		// Body is JSON data and looks like:
		// {"public_id":"Downloads/file","version":1369431906,"format":"png","resource_type":"image"}
		dec := json.NewDecoder(resp.Body)
		upInfo := new(uploadResponse)
		if err := dec.Decode(upInfo); err != nil {
			return err
		}
		fmt.Println(upInfo.PublicId)
	} else {
		return errors.New("Request error: " + resp.Status)
	}

	return nil
}

// Upload a file or a set of files in the cloud. Set ramdomPublicId to true
// to let the service generate a unique random public id. If set to false,
// the resource's public id is computed using the absolute path to the file.
// Set rtype to the target resource type, e.g. image or raw file.
//
// For example, a raw file /tmp/css/default.css will be stored with a public
// name of css/default.css (raw file keeps its extension), but an image file
// /tmp/images/logo.png will be stored as images/logo.
//
// If the source path is a directory, all files are recursively uploaded to
// the cloud service.
func (s *Service) Upload(path string, randomPublicId bool, rtype ResourceType) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	s.uploadResType = rtype
	if info.IsDir() {
		if err := filepath.Walk(path, s.walkIt); err != nil {
			return err
		}
	} else {
		if err := s.uploadFile(path, randomPublicId); err != nil {
			return err
		}
	}
	return nil
}

// Url returns the complete access path in the cloud to the
// resource designed by publicId or the empty string if
// no match.
func (s *Service) Url(publicId string) string {
	return ""
}

func handleHttpResponse(resp *http.Response) (map[string]interface{}, error) {
	if resp == nil {
		return nil, errors.New("nil http response")
	}
	dec := json.NewDecoder(resp.Body)
	var msg interface{}
	if err := dec.Decode(&msg); err != nil {
		return nil, err
	}
	m := msg.(map[string]interface{})
	if resp.StatusCode != http.StatusOK {
		// JSON error looks like {"error":{"message":"Missing required parameter - public_id"}}
		if e, ok := m["error"]; ok {
			return nil, errors.New(e.(map[string]interface{})["message"].(string))
		}
		return nil, errors.New(resp.Status)
	}
	return m, nil
}

// Delete deletes a resource uploaded to Cloudinary.
func (s *Service) Delete(publicId string, rtype ResourceType) error {
	// TODO: also delete resource entry from database (if used)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	data := url.Values{
		"api_key":   []string{s.apiKey},
		"public_id": []string{publicId},
		"timestamp": []string{timestamp},
	}

	// Signature
	hash := sha1.New()
	part := fmt.Sprintf("public_id=%s&timestamp=%s%s", publicId, timestamp, s.apiSecret)
	io.WriteString(hash, part)
	data.Set("signature", fmt.Sprintf("%x", hash.Sum(nil)))

	rt := imageType
	if rtype == RawType {
		rt = rawType
	}
	resp, err := http.PostForm(fmt.Sprintf("%s/%s/%s/destroy/", baseUploadUrl, s.cloudName, rt), data)
	if err != nil {
		return err
	}

	m, err := handleHttpResponse(resp)
	if err != nil {
		return err
	}
	if e, ok := m["result"]; ok {
		fmt.Println(e.(string))
	}
	return nil
}
