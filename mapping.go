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
