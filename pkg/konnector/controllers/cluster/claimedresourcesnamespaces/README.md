# claimedresourcesnamespaces

When using claimed resources, it can be situations where resource does not yet exists in the consumer cluster OR
we are operating in cluster scoped mode, and hence no resource namespace is created. In scenarios like this,
`APIServiceNamespace` object is never creates, and hence backend will not create appropriate RBAC for the consumer to be able to
access the resources.

It will watch same GVR resources, as claimed resources (hence the name of the controller), and will create `APIServiceNamespace` objects,
if object is created in the consumer cluster. 

We can't add this logic into `claimedresource` controller, as it never gets to healthy state until provider has right rbac configured and 
consumer can access the resources and hence start the informers. Alternative to this is create `APIServiceNamespace` objects outside reconile
loop, but that would be against the controller pattern.