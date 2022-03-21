package search

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/MakeNowJust/heredoc"
	"github.com/cli/cli/v2/pkg/httpmock"
	"github.com/stretchr/testify/assert"
)

var query = Query{
	Keywords: []string{"keyword"},
	Kind:     "repositories",
	Limit:    30,
	Order:    "stars",
	Sort:     "desc",
	Qualifiers: Qualifiers{
		Stars: ">=5",
		Topic: []string{"topic"},
	},
}

func TestSearcherRepositories(t *testing.T) {
	values := url.Values{
		"page":     []string{"1"},
		"per_page": []string{"30"},
		"order":    []string{"stars"},
		"sort":     []string{"desc"},
		"q":        []string{"keyword stars:>=5 topic:topic"},
	}

	tests := []struct {
		name      string
		host      string
		query     Query
		result    RepositoriesResult
		wantErr   bool
		errMsg    string
		httpStubs func(*httpmock.Registry)
	}{
		{
			name:  "searches repositories",
			query: query,
			result: RepositoriesResult{
				IncompleteResults: false,
				Items:             []Repository{{Name: "test"}},
				Total:             1,
			},
			httpStubs: func(reg *httpmock.Registry) {
				reg.Register(
					httpmock.QueryMatcher("GET", "search/repositories", values),
					httpmock.JSONResponse(RepositoriesResult{
						IncompleteResults: false,
						Items:             []Repository{{Name: "test"}},
						Total:             1,
					}),
				)
			},
		},
		{
			name:  "searches repositories for enterprise host",
			host:  "enterprise.com",
			query: query,
			result: RepositoriesResult{
				IncompleteResults: false,
				Items:             []Repository{{Name: "test"}},
				Total:             1,
			},
			httpStubs: func(reg *httpmock.Registry) {
				reg.Register(
					httpmock.QueryMatcher("GET", "api/v3/search/repositories", values),
					httpmock.JSONResponse(RepositoriesResult{
						IncompleteResults: false,
						Items:             []Repository{{Name: "test"}},
						Total:             1,
					}),
				)
			},
		},
		{
			name:  "paginates results",
			query: query,
			result: RepositoriesResult{
				IncompleteResults: false,
				Items:             []Repository{{Name: "test"}, {Name: "cli"}},
				Total:             2,
			},
			httpStubs: func(reg *httpmock.Registry) {
				firstReq := httpmock.QueryMatcher("GET", "search/repositories", values)
				firstRes := httpmock.JSONResponse(RepositoriesResult{
					IncompleteResults: false,
					Items:             []Repository{{Name: "test"}},
					Total:             2,
				},
				)
				firstRes = httpmock.WithHeader(firstRes, "Link", `<https://api.github.com/search/repositories?page=2&per_page=100&q=org%3Agithub>; rel="next"`)
				secondReq := httpmock.QueryMatcher("GET", "search/repositories", url.Values{
					"page":     []string{"2"},
					"per_page": []string{"29"},
					"order":    []string{"stars"},
					"sort":     []string{"desc"},
					"q":        []string{"keyword stars:>=5 topic:topic"},
				},
				)
				secondRes := httpmock.JSONResponse(RepositoriesResult{
					IncompleteResults: false,
					Items:             []Repository{{Name: "cli"}},
					Total:             2,
				},
				)
				reg.Register(firstReq, firstRes)
				reg.Register(secondReq, secondRes)
			},
		},
		{
			name:    "handles search errors",
			query:   query,
			wantErr: true,
			errMsg: heredoc.Doc(`
        Invalid search query "keyword stars:>=5 topic:topic".
        "blah" is not a recognized date/time format. Please provide an ISO 8601 date/time value, such as YYYY-MM-DD.`),
			httpStubs: func(reg *httpmock.Registry) {
				reg.Register(
					httpmock.QueryMatcher("GET", "search/repositories", values),
					httpmock.WithHeader(
						httpmock.StatusStringResponse(422,
							`{
                "message":"Validation Failed",
                "errors":[
                  {
                    "message":"\"blah\" is not a recognized date/time format. Please provide an ISO 8601 date/time value, such as YYYY-MM-DD.",
                    "resource":"Search",
                    "field":"q",
                    "code":"invalid"
                  }
                ],
                "documentation_url":"https://docs.github.com/v3/search/"
              }`,
						), "Content-Type", "application/json"),
				)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := &httpmock.Registry{}
			defer reg.Verify(t)
			if tt.httpStubs != nil {
				tt.httpStubs(reg)
			}
			client := &http.Client{Transport: reg}
			if tt.host == "" {
				tt.host = "github.com"
			}
			searcher := NewSearcher(client, tt.host)
			result, err := searcher.Repositories(tt.query)
			if tt.wantErr {
				assert.EqualError(t, err, tt.errMsg)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.result, result)
		})
	}
}

func TestSearcherURL(t *testing.T) {
	tests := []struct {
		name  string
		host  string
		query Query
		url   string
	}{
		{
			name:  "outputs encoded query url",
			query: query,
			url:   "https://github.com/search?order=stars&q=keyword+stars%3A%3E%3D5+topic%3Atopic&sort=desc&type=repositories",
		},
		{
			name:  "supports enterprise hosts",
			host:  "enterprise.com",
			query: query,
			url:   "https://enterprise.com/search?order=stars&q=keyword+stars%3A%3E%3D5+topic%3Atopic&sort=desc&type=repositories",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.host == "" {
				tt.host = "github.com"
			}
			searcher := NewSearcher(nil, tt.host)
			assert.Equal(t, tt.url, searcher.URL(tt.query))
		})
	}
}