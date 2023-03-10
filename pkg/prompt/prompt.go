package prompt

import (
	"context"

	"github.com/AlecAivazis/survey/v2"
	"github.com/common-fate/common-fate/pkg/types"
)

// Prompt the user to select a handler
func Handler(ctx context.Context, cf *types.ClientWithResponses) (*types.TGHandler, error) {
	res, err := cf.AdminListHandlersWithResponse(ctx)
	if err != nil {
		return nil, err
	}

	var handlers []string
	handlerMap := make(map[string]types.TGHandler)

	for _, h := range res.JSON200.Res {
		handlerMap[h.Id] = h
		handlers = append(handlers, h.Id)
	}
	var id string
	err = survey.AskOne(&survey.Select{Message: "Select a handler", Options: handlers}, &id)
	if err != nil {
		return nil, err
	}
	handler := handlerMap[id]
	return &handler, nil
}

// Prompt the user to select a target group
func TargetGroup(ctx context.Context, cf *types.ClientWithResponses) (*types.TargetGroup, error) {
	res, err := cf.AdminListTargetGroupsWithResponse(ctx)
	if err != nil {
		return nil, err
	}

	var handlers []string
	handlerMap := make(map[string]types.TargetGroup)

	for _, h := range res.JSON200.TargetGroups {
		handlerMap[h.Id] = h
		handlers = append(handlers, h.Id)
	}
	var id string
	err = survey.AskOne(&survey.Select{Message: "Select a target group", Options: handlers}, &id)
	if err != nil {
		return nil, err
	}
	handler := handlerMap[id]
	return &handler, nil
}
