//revive:disable:dot-imports
package eval_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/eval"
)

var _ = Describe("prePRStubDesc", func() {
	When("the tool name has the expected api_v1_Service_Method format", func() {
		It("should return MethodName — see (api.v1.description)", func() {
			result := eval.PRStubDesc("api_v1_PageManagementService_CreatePage")
			Expect(result).To(ContainSubstring("CreatePage"))
			Expect(result).To(ContainSubstring("see (api.v1.description)"))
		})
	})

	When("the tool name has too few parts", func() {
		It("should return an empty string", func() {
			result := eval.PRStubDesc("short")
			Expect(result).To(Equal(""))
		})
	})
})

var _ = Describe("isFromService", func() {
	When("the tool name contains the service name", func() {
		It("should return true", func() {
			Expect(eval.IsFromService("api_v1_PageManagementService_CreatePage", "PageManagementService")).To(BeTrue())
		})
	})

	When("the tool name does not contain the service name", func() {
		It("should return false", func() {
			Expect(eval.IsFromService("api_v1_SearchService_Search", "PageManagementService")).To(BeFalse())
		})
	})
})

var _ = Describe("ToPrePR", func() {
	var (
		postSurface eval.ToolSurface
		preSurface  eval.ToolSurface
	)

	BeforeEach(func() {
		postSurface = eval.ToolSurface{
			Label: "post",
			Tools: []eval.ToolDef{
				{Name: "api_v1_PageManagementService_CreatePage", Description: "Creates a new wiki page with an identifier."},
				{Name: "api_v1_SearchService_Search", Description: "Searches the wiki content."},
				{Name: "api_v1_SurveyService_GetSurvey", Description: "Gets a survey."},
			},
		}
		preSurface = eval.ToPrePR(postSurface)
	})

	It("should label the result as pre-PR", func() {
		Expect(preSurface.Label).To(Equal("pre-PR"))
	})

	It("should replace stub service descriptions with method-stub format", func() {
		var createPageTool *eval.ToolDef
		for i := range preSurface.Tools {
			if preSurface.Tools[i].Name == "api_v1_PageManagementService_CreatePage" {
				createPageTool = &preSurface.Tools[i]
				break
			}
		}
		Expect(createPageTool).NotTo(BeNil())
		Expect(createPageTool.Description).To(ContainSubstring("see (api.v1.description)"))
	})

	It("should empty SurveyService descriptions", func() {
		var surveyTool *eval.ToolDef
		for i := range preSurface.Tools {
			if preSurface.Tools[i].Name == "api_v1_SurveyService_GetSurvey" {
				surveyTool = &preSurface.Tools[i]
				break
			}
		}
		Expect(surveyTool).NotTo(BeNil())
		Expect(surveyTool.Description).To(Equal(""))
	})

	It("should add the pre-PR extra tools", func() {
		names := make([]string, len(preSurface.Tools))
		for i, t := range preSurface.Tools {
			names[i] = t.Name
		}
		Expect(names).To(ContainElement("api_v1_ScheduledTurnService_CompleteScheduledTurn"))
	})

	It("should have more tools than the post surface (extra tools re-added)", func() {
		Expect(len(preSurface.Tools)).To(BeNumerically(">", len(postSurface.Tools)))
	})
})
