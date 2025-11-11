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
```

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

## Troubleshooting

### Common Issues

1. **Certificate validation failures**: Use `--oidc-ca-file` to specify custom CA certificates
2. **Callback URL mismatches**: Ensure the callback URL matches what's configured in your OIDC provider
3. **Token validation errors**: Check that the issuer URL matches between provider and backend configuration
4. **Network connectivity**: Verify the backend can reach the external OIDC provider

### Debug Logging

Enable debug logging to troubleshoot OIDC issues:

```bash
--v=2  # Increase verbosity for more detailed logs
```