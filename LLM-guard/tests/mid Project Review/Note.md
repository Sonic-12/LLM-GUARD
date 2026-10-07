## Important Note
RBAC must be **temporarily disabled** to run this Mid-Project Review, since the red-team test does not send an authentication token and RBAC did not exist at the time this review was originally written against.

1. In `config.yaml`, set `rbac.enabled: false`.
2. Restart the proxy.
3. Run the Mid-Project Review tests.
4. Set `rbac.enabled: true` again and restart the proxy once finished.