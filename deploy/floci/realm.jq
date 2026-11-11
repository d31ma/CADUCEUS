def audience: {
  name: "mesail-audience",
  protocol: "openid-connect",
  protocolMapper: "oidc-audience-mapper",
  config: {
    "included.custom.audience": "mesail",
    "access.token.claim": "true",
    "id.token.claim": "false",
    "introspection.token.claim": "true"
  }
};

def person($password; $realm): {
  username: .username,
  email: .email,
  emailVerified: true,
  enabled: true,
  firstName: .username,
  lastName: $realm,
  credentials: [{ type: "password", value: $password, temporary: false }],
  realmRoles: .roles
};

. as $stack
| ($stack.realms[] | select(.name == $realm)) as $entry
| {
    realm: $realm,
    enabled: true,
    displayName: $realm,
    sslRequired: "none",
    accessTokenLifespan: 60,
    revokeRefreshToken: true,
    refreshTokenMaxReuse: 0,
    roles: { realm: [$stack.realmRoles[] | { name: . }] },
    clients: [
      {
        clientId: "mesail-web",
        publicClient: true,
        standardFlowEnabled: true,
        directAccessGrantsEnabled: false,
        redirectUris: [($public + "/*")],
        webOrigins: [$public],
        attributes: { "pkce.code.challenge.method": "S256", "post.logout.redirect.uris": ($public + "/*") },
        protocolMappers: [audience]
      },
      {
        clientId: "mesail-blackbox",
        publicClient: true,
        standardFlowEnabled: false,
        directAccessGrantsEnabled: true,
        attributes: { "access.token.lifespan": "1800" },
        protocolMappers: [audience]
      },
      {
        clientId: "mesail-service",
        publicClient: false,
        secret: $secret,
        serviceAccountsEnabled: true,
        standardFlowEnabled: false,
        directAccessGrantsEnabled: false,
        protocolMappers: [audience]
      }
    ],
    users: (
      [$entry.users[] | person($password; $realm)]
      + [{
          username: "service-account-mesail-service",
          enabled: true,
          serviceAccountClientId: "mesail-service",
          realmRoles: ["mesail-service"]
        }]
    )
  }
