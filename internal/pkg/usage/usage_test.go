package usage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/psyb0t/chatz/internal/pkg/metrics"
	"github.com/psyb0t/elelem"
	"github.com/psyb0t/elelem/elelemtest/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const testModelID = "gpt"

// newDriverMock returns the generated elelem.Driver mock with the two calls
// the recorder may make on any path pre-stubbed as optional. Each test then
// sets the expectation it actually cares about, and NewMockDriver's registered
// cleanup fails the test if that call never happens — which is the job the old
// hand-rolled fake's `streamCalled` bool was doing by hand, for one method
// only.
func newDriverMock(t *testing.T) *mocks.MockDriver {
	t.Helper()

	driver := mocks.NewMockDriver(t)
	driver.EXPECT().Capabilities(mock.Anything).
		Return(elelem.Capabilities{}).Maybe()
	driver.EXPECT().TokenCounter().
		Return(elelem.DefaultTokenCounter()).Maybe()

	return driver
}

func newRecorder(t *testing.T, driver elelem.Driver) elelem.Driver {
	t.Helper()

	met, err := metrics.New()
	require.NoError(t, err)

	return Wrap(driver, met, "chat", false)
}

func TestRecorder_StreamPassthrough(t *testing.T) {
	t.Parallel()

	driver := newDriverMock(t)
	driver.EXPECT().
		Stream(mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(
			_ context.Context,
			_ elelem.DriverRequest,
			onDelta func(elelem.Delta) error,
		) (elelem.Usage, error) {
			require.NoError(t, onDelta(elelem.Delta{Text: "hi"}))

			return elelem.Usage{TokenCounts: elelem.TokenCounts{
				Prompt:     5,
				Completion: 7,
				Total:      12,
			}}, nil
		}).Once()

	rec := newRecorder(t, driver)

	var got string

	usage, err := rec.Stream(
		t.Context(),
		elelem.DriverRequest{Model: elelem.Model{ID: testModelID}},
		func(delta elelem.Delta) error {
			got += delta.Text

			return nil
		},
	)
	require.NoError(t, err)
	assert.Equal(t, "hi", got)
	assert.Equal(t, int64(12), usage.Total)
}

func TestRecorder_StreamErrorPropagates(t *testing.T) {
	t.Parallel()

	driver := newDriverMock(t)
	driver.EXPECT().
		Stream(mock.Anything, mock.Anything, mock.Anything).
		Return(elelem.Usage{}, assert.AnError).Once()

	rec := newRecorder(t, driver)

	_, err := rec.Stream(
		t.Context(),
		elelem.DriverRequest{Model: elelem.Model{ID: testModelID}},
		func(elelem.Delta) error { return nil },
	)
	require.ErrorIs(t, err, assert.AnError)
}

// The non-streaming path must be instrumented identically. Tokens are spent
// and the row is owed either way, so a Complete that only forwarded would make
// a backend's entire usage vanish from the metrics the moment someone set
// elelem.WithStreaming(false) — with nothing failing to say so.
func TestRecorder_CompleteRecordsTheSameTokensAsStream(t *testing.T) {
	t.Parallel()

	spent := elelem.Usage{TokenCounts: elelem.TokenCounts{
		Prompt:     5,
		Completion: 7,
		Total:      12,
	}}

	testCases := []struct {
		name   string
		stub   func(*mocks.MockDriver)
		invoke func(
			elelem.Driver,
			context.Context,
			func(elelem.Delta) error,
		) (elelem.Usage, error)
	}{
		{
			name: "streaming",
			stub: func(driver *mocks.MockDriver) {
				driver.EXPECT().
					Stream(mock.Anything, mock.Anything, mock.Anything).
					Return(spent, nil).Once()
			},
			invoke: func(
				driver elelem.Driver,
				ctx context.Context,
				onDelta func(elelem.Delta) error,
			) (elelem.Usage, error) {
				return driver.Stream(
					ctx,
					elelem.DriverRequest{Model: elelem.Model{ID: testModelID}},
					onDelta,
				)
			},
		},
		{
			name: "not streaming",
			stub: func(driver *mocks.MockDriver) {
				driver.EXPECT().
					Complete(mock.Anything, mock.Anything, mock.Anything).
					Return(spent, nil).Once()
			},
			invoke: func(
				driver elelem.Driver,
				ctx context.Context,
				onDelta func(elelem.Delta) error,
			) (elelem.Usage, error) {
				return driver.Complete(
					ctx,
					elelem.DriverRequest{Model: elelem.Model{ID: testModelID}},
					onDelta,
				)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			met, err := metrics.New()
			require.NoError(t, err)

			driver := newDriverMock(t)
			tc.stub(driver)

			usage, err := tc.invoke(
				Wrap(driver, met, "chat", false),
				t.Context(),
				func(elelem.Delta) error { return nil },
			)
			require.NoError(t, err)
			assert.Equal(t, int64(12), usage.Total, "usage passes through")

			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
			met.Handler().ServeHTTP(response, request)

			body := response.Body.String()
			assert.Contains(
				t,
				body,
				//nolint:lll // Prometheus series assertion must remain exact.
				`llm_tokens_total{kind="input",model="gpt",service="chatz",stage="chat"} 5`,
			)
			assert.Contains(
				t,
				body,
				//nolint:lll // Prometheus series assertion must remain exact.
				`llm_tokens_total{kind="output",model="gpt",service="chatz",stage="chat"} 7`,
			)
		})
	}
}

func TestRecorder_RecordsRetryWasteOnce(t *testing.T) {
	t.Parallel()

	met, err := metrics.New()
	require.NoError(t, err)

	driver := newDriverMock(t)
	driver.EXPECT().
		Stream(mock.Anything, mock.Anything, mock.Anything).
		Return(elelem.Usage{
			TokenCounts: elelem.TokenCounts{
				Prompt:     5,
				Completion: 7,
				Total:      12,
			},
			Retry: elelem.RetryInfo{
				WastedPromptTokens:     3,
				WastedCompletionTokens: 4,
				WastedTotalTokens:      7,
			},
		}, nil).Once()

	recorder := Wrap(driver, met, "chat", false)

	_, err = recorder.Stream(
		t.Context(),
		elelem.DriverRequest{Model: elelem.Model{ID: testModelID}},
		nil,
	)
	require.NoError(t, err)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	met.Handler().ServeHTTP(response, request)
	body := response.Body.String()
	assert.Contains(
		t,
		body,
		//nolint:lll // Prometheus series assertion must remain exact.
		`llm_tokens_total{kind="input",model="gpt",service="chatz",stage="chat"} 8`,
	)
	assert.Contains(
		t,
		body,
		//nolint:lll // Prometheus series assertion must remain exact.
		`llm_tokens_total{kind="output",model="gpt",service="chatz",stage="chat"} 11`,
	)
}

func TestWithAttribution_RoundTrip(t *testing.T) {
	t.Parallel()

	chatID := uuid.New()
	userID := uuid.New()

	ctx := WithAttribution(t.Context(), chatID, userID)

	attr, ok := attributionFromContext(ctx)
	require.True(t, ok)
	assert.Equal(t, chatID, attr.chatID)
	assert.Equal(t, userID, attr.userID)
}

func TestAttributionFromContext_Missing(t *testing.T) {
	t.Parallel()

	_, ok := attributionFromContext(t.Context())
	assert.False(t, ok)
}

func TestRecorder_ListModelsPassthrough(t *testing.T) {
	t.Parallel()

	driver := newDriverMock(t)
	driver.EXPECT().ListModels(mock.Anything).
		Return([]string{"a", "b"}, nil).Once()

	rec := newRecorder(t, driver)

	models, err := rec.ListModels(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, models)
}
