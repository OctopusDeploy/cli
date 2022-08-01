package integration

// things to test (interactive mode):
// NOTE the question flow does the matching here:
//   case-insensitive match on channel name
//   no partial match on channel name
//   case-insensitive match on project name
//   no partial match on project name

// things to test (automation mode):
// NOTE the question flow does not run here, should it?
//   case-insensitive match on channel name
//   no partial match on channel name
//   case-insensitive match on project name
//   no partial match on project name

// DESIGN QUESTION:
// In automation mode, if a user specifies "foo project" and the actual thing is "Foo PROJECT", should the
// command do a pass over the options first, and replace things with their correctly-cased versions before
// feeding into the executor, or should the executor do that?
//
// The nicest thing would be if the executor could just be a blind pass-through into the server, however
// because the octopus server doesn't support things like
// matching a project based on exact name, we have to do at least SOME client side filtering first.
// The executions API may be an exception to this rule, but in general, it holds.
