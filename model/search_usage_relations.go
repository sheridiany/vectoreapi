package model

import "context"

const searchUsageRelationBatchSize = 500

type SearchUsageRelations struct {
	AgentKeyNames    map[int]string
	UserNames        map[int]string
	EnterpriseNames  map[int]string
	UpstreamAccounts map[int]string
}

func LoadSearchUsageRelations(ctx context.Context, events []*SearchUsageEvent, includeAdmin bool) (*SearchUsageRelations, error) {
	relations := &SearchUsageRelations{
		AgentKeyNames:    make(map[int]string),
		UserNames:        make(map[int]string),
		EnterpriseNames:  make(map[int]string),
		UpstreamAccounts: make(map[int]string),
	}
	keyIDs := make([]int, 0)
	userIDs := make([]int, 0)
	enterpriseIDs := make([]int, 0)
	accountIDs := make([]int, 0)
	seenKeys := make(map[int]struct{})
	seenUsers := make(map[int]struct{})
	seenEnterprises := make(map[int]struct{})
	seenAccounts := make(map[int]struct{})
	for _, event := range events {
		if event == nil {
			continue
		}
		if event.AgentKeyID > 0 {
			if _, exists := seenKeys[event.AgentKeyID]; !exists {
				seenKeys[event.AgentKeyID] = struct{}{}
				keyIDs = append(keyIDs, event.AgentKeyID)
			}
		}
		if !includeAdmin {
			continue
		}
		if event.UserID > 0 {
			if _, exists := seenUsers[event.UserID]; !exists {
				seenUsers[event.UserID] = struct{}{}
				userIDs = append(userIDs, event.UserID)
			}
		}
		if event.EnterpriseID > 0 {
			if _, exists := seenEnterprises[event.EnterpriseID]; !exists {
				seenEnterprises[event.EnterpriseID] = struct{}{}
				enterpriseIDs = append(enterpriseIDs, event.EnterpriseID)
			}
		}
		if event.UpstreamAccountID > 0 {
			if _, exists := seenAccounts[event.UpstreamAccountID]; !exists {
				seenAccounts[event.UpstreamAccountID] = struct{}{}
				accountIDs = append(accountIDs, event.UpstreamAccountID)
			}
		}
	}

	type namedRelation struct {
		Id   int    `gorm:"column:id"`
		Name string `gorm:"column:name"`
	}
	for start := 0; start < len(keyIDs); start += searchUsageRelationBatchSize {
		end := min(start+searchUsageRelationBatchSize, len(keyIDs))
		rows := make([]namedRelation, 0, end-start)
		if err := DB.WithContext(ctx).Model(&SearchAgentKey{}).Select("id, name").Where("id IN ?", keyIDs[start:end]).Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			relations.AgentKeyNames[row.Id] = row.Name
		}
	}
	if !includeAdmin {
		return relations, nil
	}

	type userRelation struct {
		Id       int    `gorm:"column:id"`
		Username string `gorm:"column:username"`
	}
	for start := 0; start < len(userIDs); start += searchUsageRelationBatchSize {
		end := min(start+searchUsageRelationBatchSize, len(userIDs))
		rows := make([]userRelation, 0, end-start)
		if err := DB.WithContext(ctx).Model(&User{}).Select("id, username").Where("id IN ?", userIDs[start:end]).Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			relations.UserNames[row.Id] = row.Username
		}
	}
	for start := 0; start < len(enterpriseIDs); start += searchUsageRelationBatchSize {
		end := min(start+searchUsageRelationBatchSize, len(enterpriseIDs))
		rows := make([]namedRelation, 0, end-start)
		if err := DB.WithContext(ctx).Model(&Enterprise{}).Select("id, name").Where("id IN ?", enterpriseIDs[start:end]).Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			relations.EnterpriseNames[row.Id] = row.Name
		}
	}
	for start := 0; start < len(accountIDs); start += searchUsageRelationBatchSize {
		end := min(start+searchUsageRelationBatchSize, len(accountIDs))
		rows := make([]namedRelation, 0, end-start)
		if err := DB.WithContext(ctx).Model(&SearchUpstreamAccount{}).Select("id, name").Where("id IN ?", accountIDs[start:end]).Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			relations.UpstreamAccounts[row.Id] = row.Name
		}
	}
	return relations, nil
}
