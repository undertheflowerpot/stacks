package dispatcher

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"time"

	"github.com/docker/stacks/pkg/mocks"
	"github.com/golang/mock/gomock"

	"github.com/docker/docker/api/types/events"

	"github.com/docker/stacks/pkg/interfaces"
	"github.com/docker/stacks/pkg/reconciler/notifier"
)

type fakeRegisterFunc func(notifier.ObjectChangeNotifier)

func (f fakeRegisterFunc) Register(n notifier.ObjectChangeNotifier) {
	if f != nil {
		f(n)
	}
}

// MatchesRequestIDs is a gomock matcher which asserts that the actual ID used in the
// call is one of the specified IDs, and that each ID is used only once
func MatchesRequestIDs(kind interfaces.ReconcileKind, ids ...string) gomock.Matcher {
	i := &requestIDMatcher{
		kind:        kind,
		expectedIDs: map[string]bool{},
	}

	for _, id := range ids {
		i.expectedIDs[id] = false
	}
	return i
}

type requestIDMatcher struct {
	kind        interfaces.ReconcileKind
	expectedIDs map[string]bool
}

func (i *requestIDMatcher) Matches(x interface{}) bool {
	var request *interfaces.ReconcileResource
	var ok bool

	request, ok = x.(*interfaces.ReconcileResource)

	if !ok {
		return false
	}

	id := request.ID
	if request.Kind != i.kind {
		return false
	}

	if used, ok := i.expectedIDs[id]; used || !ok {
		return false
	}
	i.expectedIDs[id] = true
	return true
}

func (i *requestIDMatcher) String() string {
	return "is one of the specified reconcile request IDs (only once)"
}

var _ = Describe("Dispatcher", func() {
	var (
		mockCtrl *gomock.Controller
		// NOTE(dperny): I choose to use mocks for this test as the stand-in
		// for the the Reconciler, because this is a good use-case for mocks. I
		// want to be sure that the dispatcher calls only the specified methods
		// in the specified order with the specified arguments, and return the
		// specified result. Unlike in the Reconciler package, where the
		// implementation is decoupled from the end-result, the Dispatcher is
		// all about calling methods at the right time in the right order.
		mockReconciler *mocks.MockReconciler

		reg fakeRegisterFunc
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockReconciler = mocks.NewMockReconciler(mockCtrl)
	})

	Describe("creating a new dispatcher", func() {
		var (
			registered     bool
			registeredWith notifier.ObjectChangeNotifier
		)

		BeforeEach(func() {
			reg = func(n notifier.ObjectChangeNotifier) {
				registered = true
				registeredWith = n
			}
		})

		It("should register with the provided notifier.Register", func() {
			d := newDispatcher(mockReconciler, reg)
			Expect(registered).To(BeTrue())
			Expect(registeredWith).To(Equal(d))
		})
	})

	Describe("handling events", func() {
		var (
			d *dispatcher
		)

		BeforeEach(func() {
			d = newDispatcher(mockReconciler, reg)
		})

		It("should de-duplicate events", func() {
			// we should only get 1 call to Reconcile, even though we have 11
			// events for the same stack
			numberOfDuplicates := 11
			// create an event channel, with a buffer, and fill the buffer up
			eventC := make(chan interface{}, numberOfDuplicates)
			actor := events.Actor{
				ID: "someID",
			}

			for i := 0; i < numberOfDuplicates; i++ {
				eventC <- events.Message{
					Type:   interfaces.ReconcileStack,
					Action: "update",
					Actor:  actor,
				}
			}

			nr := NewRequest(interfaces.ReconcileStack, "someID")
			// we're expecting only 1 call to Reconcile
			mockReconciler.EXPECT().Reconcile(
				gomock.Eq(nr),
			).Return(nil)

			// TODO(dperny): this launches a goroutine, and goroutines in tests
			// are super duper unreliable and flaky. There must be a better
			// design here, but I don't know what it is.
			time.AfterFunc(3*time.Second, func() {
				close(eventC)
			})

			// now run HandleEvents
			err := d.HandleEvents(eventC)

			Expect(err).ToNot(HaveOccurred())

			// the mock will error if we try to call it more than once
		})

		It("should process events in order of Stacks, Networks, Secrets, Configs, and Services", func() {
			// first, create a channel
			// TODO(dperny): 32 is just a random choice, pick something better
			eventC := make(chan interface{}, 32)

			type objTuple struct {
				kind interfaces.ReconcileKind
				id   string
			}

			// to make this test work, we're going to start with this slice,
			// orderIn, which will define the event types we push to the
			// channel in the order they should be serviced. we're going to go
			// forward (stack network secret config service) and then backward
			// (service config secret network stack)
			orderIn := []objTuple{
				{
					kind: interfaces.ReconcileStack,
					id:   "stack1",
				}, {
					kind: interfaces.ReconcileNetwork,
					id:   "network1",
				}, {
					kind: interfaces.ReconcileSecret,
					id:   "secret1",
				}, {
					kind: interfaces.ReconcileConfig,
					id:   "config1",
				}, {
					kind: interfaces.ReconcileService,
					id:   "service1",
				}, {
					kind: interfaces.ReconcileService,
					id:   "service2",
				}, {
					kind: interfaces.ReconcileConfig,
					id:   "config2",
				}, {
					kind: interfaces.ReconcileSecret,
					id:   "secret2",
				}, {
					kind: interfaces.ReconcileNetwork,
					id:   "network2",
				}, {
					kind: interfaces.ReconcileStack,
					id:   "stack2",
				},
			}

			// now, generate and write out all of these events
			for _, obj := range orderIn {
				eventC <- events.Message{
					Type:   obj.kind,
					Action: "update",
					Actor: events.Actor{
						ID: obj.id,
					},
				}
			}

			// to avoid the use of goroutines in this test, we'll add a
			// function call in each gomock call. when all of the calls to
			// reconcile have been exhausted, we will close the channel so that
			// the dispatcher exits
			callCount := 0
			closeWhenProcessed := func(request *interfaces.ReconcileResource) {
				callCount++
				if callCount >= len(orderIn) {
					close(eventC)
				}
			}

			// now, the tricky part is making sure that the events are handled
			// in order, when the actual order of each pair is irrelevant. to
			// do this, we're going to leverage 3 different things:
			//
			// * gomock's InOrder function, which lets us set the order of
			//   expected calls
			// * gomock's Times method, which lets us set the number of
			//   expected calls
			// * a custom gomock.Matcher, which lets us create a set of IDs,
			//   which we expect to match only once.
			//
			// I have tested that this rather complex mocking expectation code
			// works by modifying the code to get different failures:
			//
			// * reordering the dispatcher deliberately causes a failure
			// * passing the same ID twice causes a failure
			// * passing an unexpected ID causes a failure
			gomock.InOrder(
				mockReconciler.EXPECT().Reconcile(
					MatchesRequestIDs(interfaces.ReconcileStack, "stack1", "stack2"),
				).Do(closeWhenProcessed).Return(nil).Times(2),
				mockReconciler.EXPECT().Reconcile(
					MatchesRequestIDs(interfaces.ReconcileNetwork, "network1", "network2"),
				).Do(closeWhenProcessed).Return(nil).Times(2),
				mockReconciler.EXPECT().Reconcile(
					MatchesRequestIDs(interfaces.ReconcileSecret, "secret1", "secret2"),
				).Do(closeWhenProcessed).Return(nil).Times(2),
				mockReconciler.EXPECT().Reconcile(
					MatchesRequestIDs(interfaces.ReconcileConfig, "config1", "config2"),
				).Do(closeWhenProcessed).Return(nil).Times(2),
				mockReconciler.EXPECT().Reconcile(
					MatchesRequestIDs(interfaces.ReconcileService, "service1", "service2"),
				).Do(closeWhenProcessed).Return(nil).Times(2),
			)

			// now fire up the dispatcher and set it to work
			err := d.HandleEvents(eventC)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})
})
