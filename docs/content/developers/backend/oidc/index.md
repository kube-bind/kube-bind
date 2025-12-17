---
title: OIDC Authentication
weight: 30
---

# OIDC Authentication

The kube-bind backend supports OpenID Connect (OIDC) authentication for securing API access. There are two modes of operation: external OIDC providers and an embedded OIDC provider for development.

## External OIDC Provider (Production)

For production deployments, use an external OIDC provider such as:
- Dex
- Keycloak 
- Auth0
- Google
- Microsoft Azure AD
- Any OIDC-compliant provider

### Configuration

Configure the backend to use an external OIDC provider with these flags:

```bash
--oidc-type=external
--oidc-issuer-url=https://your-oidc-provider.com
--oidc-issuer-client-id=your-client-id
--oidc-issuer-client-secret=your-client-secret
--oidc-callback-url=https://your-backend.com/callback
--oidc-authorize-url=https://your-oidc-provider.com/auth
--oidc-ca-file=/path/to/ca-bundle.pem  # Optional: for custom CA certificates
--oidc-allowed-groups=group1,group2,group3  # Optional: restrict access to specific groups
--oidc-allowed-users=user1,user2,user3      # Optional: restrict access to specific users
```

**Note**: When using external OIDC providers, at least one of `--oidc-allowed-groups` or `--oidc-allowed-users` must be specified for security reasons.

### External OIDC Flow

1. User initiates authentication by accessing a protected endpoint
2. Backend redirects user to the external OIDC provider's authorization endpoint
3. User authenticates with the OIDC provider
4. OIDC provider redirects back to the backend's callback URL with authorization code
5. Backend exchanges authorization code for ID token and access token
6. Backend validates the tokens and establishes user session
7. User can now access protected APIs with valid session

## Embedded OIDC Provider (Development Only)

For development and testing purposes, kube-bind includes an embedded OIDC provider that eliminates the need to set up an external authentication service.

### Configuration

Configure the backend to use the embedded OIDC provider:

```bash
--oidc-type=embedded
--oidc-callback-url=https://your-backend.com/callback
--oidc-issuer-url=https://your-backend.com  # The backend serves as the OIDC provider
--oidc-allowed-groups=group1,group2,group3  # Optional: restrict access to specific groups
--oidc-allowed-users=user1,user2,user3      # Optional: restrict access to specific users
```

### Embedded OIDC Flow

1. Backend starts up and initializes the embedded OIDC server using mockoidc
2. The embedded OIDC server automatically generates:
   - Client ID and client secret
   - Signing keys for tokens
   - OIDC discovery endpoints (`/.well-known/openid_configuration`)
3. User initiates authentication by accessing a protected endpoint
4. Backend redirects user to its own embedded OIDC authorization endpoint
5. User authenticates with the embedded provider (typically accepts any credentials for development)
6. Embedded OIDC provider redirects back to the backend's callback URL
7. Backend validates the tokens (self-issued) and establishes user session
8. User can now access protected APIs with valid session

### Development Mode Warning

When using embedded OIDC, the backend displays a prominent warning message at startup to remind developers that this mode is not suitable for production use.

## Security Considerations

### External OIDC (Production)
- Use HTTPS for all endpoints
- Validate OIDC provider certificates
- Use strong client secrets
- Configure proper redirect URI restrictions
- Monitor for security updates to the OIDC provider

### Embedded OIDC (Development Only)
- **NEVER use in production** - provides no real security
- Accepts any credentials for authentication
- Intended only for local development and testing
- Does not persist user data between restarts

## Access Control

The kube-bind backend provides fine-grained access control through OIDC group and user-based authorization. When a user successfully authenticates via OIDC, the backend automatically creates RBAC resources to control access to cluster binding operations.

### Access Control Flags

- `--oidc-allowed-groups`: List of OIDC groups allowed to access bindings inside the cluster. Users must be members of at least one of these groups to gain access.
- `--oidc-allowed-users`: List of specific OIDC users allowed to access bindings inside the cluster.

**Important**: For external OIDC providers, at least one of these flags must be specified. This validation ensures that access control is properly configured in production environments.

### How Access Control Works

1. **RBAC Resource Creation**: For each cluster, the controller automatically creates:
   - A `ClusterRole` named `kube-bind-oidc-user` with permissions to perform binding operations
   - A `ClusterRoleBinding` that binds the role to the specified groups and users

2. **Group-based Access**: Users who are members of groups listed in `--oidc-allowed-groups` are granted access via RBAC subject entries of kind `Group`.

3. **User-based Access**: Individual users listed in `--oidc-allowed-users` are granted access via RBAC subject entries of kind `User`.

4. **Embedded OIDC Special Behavior**: When using embedded OIDC (`--oidc-type=embedded`), the `system:authenticated` group is automatically added to the allowed groups list, providing access to all authenticated users during development.

### Example Configuration

```bash
# Restrict access to specific groups and users
--oidc-allowed-groups=kube-bind-admins,platform-team
--oidc-allowed-users=admin@example.com,operator@example.com
```

This configuration would create RBAC bindings allowing:
- All members of the `kube-bind-admins` group
- All members of the `platform-team` group  
- The specific users `admin@example.com` and `operator@example.com`

## Troubleshooting

### Common Issues

1. **Certificate validation failures**: Use `--oidc-ca-file` to specify custom CA certificates
2. **Callback URL mismatches**: Ensure the callback URL matches what's configured in your OIDC provider
3. **Token validation errors**: Check that the issuer URL matches between provider and backend configuration
4. **Network connectivity**: Verify the backend can reach the external OIDC provider
5. **Access control validation errors**: When using external OIDC, ensure at least one of `--oidc-allowed-groups` or `--oidc-allowed-users` is specified

### Debug Logging

Enable debug logging to troubleshoot OIDC issues:

```bash
--v=2  # Increase verbosity for more detailed logs
```