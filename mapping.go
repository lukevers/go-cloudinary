package cloudinary

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type ListUploadMappingsOutput struct {
	Mappings []Mapping `json:"mappings"`
}

type Mapping struct {
	Folder   string `json:"folder"`
	Template string `json:"template"`
}

func (s *Service) ListUploadMappings() (*ListUploadMappingsOutput, error) {
	uri := fmt.Sprintf("%s/upload_mappings", s.adminURI.String())
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	dec := json.NewDecoder(resp.Body)
	output := &ListUploadMappingsOutput{}
	if err := dec.Decode(output); err != nil {
		return nil, err
	}

	return output, nil
}

type CreateUploadMappingInput struct {
	Mapping *Mapping
}

type CreateUploadMappingOutput struct {
	Message string `json:"message"`
}

func (s *Service) CreateUploadMapping(input *CreateUploadMappingInput) (*CreateUploadMappingOutput, error) {
	uri := fmt.Sprintf(
		"%s/upload_mappings?folder=%s&template=%s",
		s.adminURI.String(),
		input.Mapping.Folder,
		input.Mapping.Template,
	)

	req, err := http.NewRequest("POST", uri, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	dec := json.NewDecoder(resp.Body)
	output := &CreateUploadMappingOutput{}
	if err := dec.Decode(output); err != nil {
		return nil, err
	}

	return output, nil
}

type DeleteUploadMappingInput struct {
	Folder string
}

type DeleteUploadMappingOutput struct {
	Message string `json:"message"`
}

func (s *Service) DeleteUploadMapping(input *DeleteUploadMappingInput) (*DeleteUploadMappingOutput, error) {
	uri := fmt.Sprintf(
		"%s/upload_mappings?folder=%s",
		s.adminURI.String(),
		input.Folder,
	)

	req, err := http.NewRequest("DELETE", uri, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	dec := json.NewDecoder(resp.Body)
	output := &DeleteUploadMappingOutput{}
	if err := dec.Decode(output); err != nil {
		return nil, err
	}

	return output, nil
}
