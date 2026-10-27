package notify

// GraphQL documents follow the upstream GitHub provider.
const (
	githubQueryDiscussion    = "query($owner: String!, $repo: String!, $number: Int!) { repository(owner: $owner, name: $repo) { discussion(number: $number) { id } } }"
	githubQueryCategories    = "query($owner: String!, $repo: String!) { repository(owner: $owner, name: $repo) { id discussionCategories(first: 100) { nodes { id name slug } } } }"
	githubMutationComment    = "mutation($id: ID!, $body: String!) { addDiscussionComment(input: {discussionId: $id, body: $body}) { comment { id } } }"
	githubMutationDiscussion = "mutation($repo: ID!, $category: ID!, $title: String!, $body: String!) { createDiscussion(input: {repositoryId: $repo, categoryId: $category, title: $title, body: $body}) { discussion { number } } }"
)
