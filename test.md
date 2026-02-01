# GitVigil API Tests

## Health
```bash
curl http://localhost:8080/health
```

## Repositories
```bash
curl http://localhost:8080/api/v1/repositories
```

```bash
curl http://localhost:8080/api/v1/repositories/1
```

## Installations
```bash
curl http://localhost:8080/api/v1/installations
```

```bash
curl http://localhost:8080/api/v1/installations/1
```

## Stats
```bash
curl http://localhost:8080/api/v1/stats
```

## Scorecard
```bash
curl "http://localhost:8080/scorecard?repo=HarshPatel5940/gitvigil"
```
