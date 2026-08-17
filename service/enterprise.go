package service

import "github.com/QuantumNous/new-api/model"

func GetEnterprise(id int) (*model.Enterprise, error) {
	return model.GetEnterpriseByID(id)
}

type CreateEnterpriseInput struct {
	Name                string
	Code                string
	RegistrationEnabled *bool
	RegistrationMode    string
}

func CreateEnterprise(input CreateEnterpriseInput) (*model.Enterprise, error) {
	enterprise, err := model.NewEnterprise(input.Name, input.Code)
	if err != nil {
		return nil, err
	}
	if input.RegistrationEnabled != nil {
		enterprise.RegistrationEnabled = *input.RegistrationEnabled
	}
	if input.RegistrationMode != "" {
		enterprise.RegistrationMode = input.RegistrationMode
	}
	if err := enterprise.Validate(); err != nil {
		return nil, err
	}
	if err := enterprise.Insert(); err != nil {
		return nil, err
	}
	return enterprise, nil
}

func ListEnterprises(startIdx, num int) ([]*model.Enterprise, int64, error) {
	return model.GetAllEnterprises(startIdx, num)
}

type UpdateEnterpriseInput struct {
	ID                  int
	Name                *string
	Code                *string
	Status              *int
	RegistrationEnabled *bool
	RegistrationMode    *string
}

func UpdateEnterprise(input UpdateEnterpriseInput) (*model.Enterprise, error) {
	enterprise, err := model.GetEnterpriseByID(input.ID)
	if err != nil {
		return nil, err
	}
	if input.Name != nil {
		enterprise.Name = *input.Name
	}
	if input.Code != nil {
		enterprise.Code = *input.Code
	}
	if input.Status != nil {
		enterprise.Status = *input.Status
	}
	if input.RegistrationEnabled != nil {
		enterprise.RegistrationEnabled = *input.RegistrationEnabled
	}
	if input.RegistrationMode != nil {
		enterprise.RegistrationMode = *input.RegistrationMode
	}
	if err := enterprise.Update(); err != nil {
		return nil, err
	}
	return enterprise, nil
}

type RegistrationEnterpriseOption struct {
	ID               int    `json:"id"`
	Name             string `json:"name"`
	Code             string `json:"code"`
	RegistrationMode string `json:"registration_mode"`
}

func ListRegistrationEnterprises() ([]RegistrationEnterpriseOption, error) {
	enterprises, err := model.GetRegistrationEnterprises()
	if err != nil {
		return nil, err
	}
	options := make([]RegistrationEnterpriseOption, 0, len(enterprises))
	for _, enterprise := range enterprises {
		options = append(options, RegistrationEnterpriseOption{
			ID: enterprise.Id, Name: enterprise.Name, Code: enterprise.Code,
			RegistrationMode: enterprise.RegistrationMode,
		})
	}
	return options, nil
}
