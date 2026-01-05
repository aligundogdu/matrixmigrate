# MatrixMigrate

A CLI tool for migrating from Mattermost to Matrix Synapse with multi-step, resumable migration support.

## Features

- **Multi-step Migration**: Migrate users, teams, channels, and memberships in organized steps
- **SSH Tunnel Support**: Securely connect to remote servers via SSH port forwarding
- **Flexible SSH Authentication**: Support for both SSH key and password-based authentication
- **Auto-discovery**: Automatically reads Mattermost database credentials from `config.json`
- **Flexible Matrix Auth**: Login with username/password or use existing admin token
- **Beautiful TUI**: Interactive terminal UI powered by Bubble Tea with styled menus
- **Multi-language Support**: English (default) and Turkish interfaces
- **Detailed Connection Tests**: Step-by-step connection diagnostics for precise troubleshooting
- **Resumable**: Checkpoint-based migration that can be paused and resumed
- **Mapping Files**: Generates mapping files to track Mattermost → Matrix entity relationships

## Installation

```bash
go install github.com/aligundogdu/matrixmigrate/cmd/matrixmigrate@latest
```

Or build from source:

```bash
git clone https://github.com/aligundogdu/matrixmigrate.git
cd matrixmigrate
go build -o matrixmigrate ./cmd/matrixmigrate
```

## Configuration

1. Copy the example configuration:
   ```bash
   cp config.example.yaml config.yaml
   ```

2. Edit `config.yaml` with your server details:

### SSH Key Authentication (Recommended)

```yaml
mattermost:
  ssh:
    host: "mattermost.example.com"
    user: "admin"
    key_path: "~/.ssh/id_rsa"
  config_path: "/opt/mattermost/config/config.json"

matrix:
  ssh:
    host: "matrix.example.com"
    user: "admin"
    key_path: "~/.ssh/id_rsa"
  auth:
    username: "admin"
    password_env: "MATRIX_ADMIN_PASSWORD"
  homeserver: "example.com"
```

### SSH Password Authentication

```yaml
mattermost:
  ssh:
    host: "mattermost.example.com"
    user: "root"
    password_env: "MM_SSH_PASSWORD"  # Uses environment variable
  config_path: "/opt/mattermost/config/config.json"

matrix:
  ssh:
    host: "matrix.example.com"
    user: "root"
    password_env: "MX_SSH_PASSWORD"
  auth:
    username: "admin"
    password_env: "MATRIX_ADMIN_PASSWORD"
  homeserver: "example.com"
```

3. Set environment variables:
   ```bash
   # For SSH password authentication
   export MM_SSH_PASSWORD="your-mattermost-ssh-password"
   export MX_SSH_PASSWORD="your-matrix-ssh-password"
   
   # For Matrix admin login
   export MATRIX_ADMIN_PASSWORD="your-admin-password"
   ```

### How It Works

**Mattermost**: The tool connects via SSH and reads `/opt/mattermost/config/config.json` to get database credentials. No manual database configuration needed!

**Matrix**: The tool logs in with username/password to get an access token. Alternatively, you can provide an existing admin token via `MATRIX_ADMIN_TOKEN` environment variable.

## Usage

### Interactive Mode (TUI)

```bash
# Start with default language (English)
./matrixmigrate

# Start with Turkish interface
./matrixmigrate --lang tr
```

### Batch Mode

```bash
# Run specific steps
./matrixmigrate export assets
./matrixmigrate import assets
./matrixmigrate export memberships
./matrixmigrate import memberships

# Run with specific config
./matrixmigrate --config ./config.yaml export assets
```

### Test Connections

The connection test provides detailed step-by-step diagnostics:

```bash
# Test all connections
./matrixmigrate test all

# Test individual connections
./matrixmigrate test mattermost
./matrixmigrate test matrix
```

**Test Output Example:**
```
📋 Configuration
   ✓ Configuration file loaded (config.yaml found and parsed)
   ✓ Data directories accessible (Assets: ./data/assets, Mappings: ./data/mappings)

🗄️ Mattermost
   ✓ SSH configuration (Password auth via $MM_SSH_PASSWORD)
   ✓ SSH connection (root@mattermost.example.com:22)
   ✓ Mattermost config.json (/opt/mattermost/config/config.json)
   ✓ Database connection (150 users, 12 teams, 87 channels)

🔷 Matrix
   ✓ SSH configuration (Key: ~/.ssh/id_rsa)
   ✓ SSH connection (admin@matrix.example.com:22)
   ✓ API authentication (Login as admin via $MATRIX_ADMIN_PASSWORD)
   ✓ API connection (Homeserver: example.com)

✓ All connection tests passed!
```

### Check Status

```bash
./matrixmigrate status
```

## Migration Steps

| Step | Command | Description |
|------|---------|-------------|
| 1a | `export assets` | Export users, teams, channels from Mattermost |
| 1b | `import assets` | Create users, spaces, rooms in Matrix |
| 2a | `export memberships` | Export team/channel memberships from Mattermost |
| 2b | `import memberships` | Apply memberships in Matrix |

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Local Machine                            │
│  ┌─────────────┐  ┌──────────┐  ┌─────────────────────────┐│
│  │ MatrixMigrate│  │ Config   │  │ Data Store              ││
│  │     CLI     │──│  YAML    │  │ - assets/*.json.gz      ││
│  └──────┬──────┘  └──────────┘  │ - mappings/*.json       ││
│         │                       │ - state.json            ││
│         │                       └─────────────────────────┘│
└─────────┼───────────────────────────────────────────────────┘
          │
    ┌─────┴─────┐
    │           │
    ▼           ▼
┌────────┐  ┌────────┐
│Mattermost│  │ Matrix │
│SSH (key/ │  │SSH (key/│
│password) │  │password)│
│    ↓     │  │   ↓    │
│config.json│  │  API   │
│    ↓     │  │   ↓    │
│PostgreSQL│  │Login/Token
└────────┘  └────────┘
```

## Mattermost → Matrix Mapping

| Mattermost | Matrix |
|------------|--------|
| Team | Space |
| Channel | Room |
| User | User |
| Team Membership | Space Membership |
| Channel Membership | Room Membership |

## Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `MATRIX_ADMIN_PASSWORD` | Matrix admin password for login | Yes (if using auth) |
| `MATRIX_ADMIN_TOKEN` | Alternative: existing admin token | No |
| `MM_SSH_PASSWORD` | Mattermost SSH password | No (if using key) |
| `MX_SSH_PASSWORD` | Matrix SSH password | No (if using key) |
| `SSH_KEY_PASSPHRASE` | SSH key passphrase (if encrypted) | No |

## Troubleshooting

Use `./matrixmigrate test all` to identify exactly where the connection fails.

### SSH Connection Failed
- For key auth: Ensure SSH key is properly configured and has correct permissions
- For password auth: Check that the password environment variable is set
- Verify the SSH port is correct (default: 22)

### Mattermost Config Not Found
- Check the `config_path` in your config.yaml
- Try different paths: `/opt/mattermost/config/config.json`, `/opt/mattermost/config.json`
- Ensure the SSH user has read access to the file

### Matrix Login Failed
- Verify the admin username and password
- Check if the Matrix homeserver supports password login
- Ensure the user has admin privileges

### Database Connection Failed
- The tool reads credentials from Mattermost's config.json automatically
- Ensure PostgreSQL is running and accessible from localhost on the Mattermost server

## License

MIT License

---

# MatrixMigrate (Türkçe)

Mattermost'tan Matrix Synapse'a çok adımlı, devam ettirilebilir taşıma desteği sunan bir CLI aracı.

## Özellikler

- **Çok Adımlı Taşıma**: Kullanıcıları, takımları, kanalları ve üyelikleri düzenli adımlarla taşıyın
- **SSH Tünel Desteği**: SSH port yönlendirme ile uzak sunuculara güvenli bağlantı
- **Esnek SSH Kimlik Doğrulama**: SSH anahtarı veya şifre tabanlı kimlik doğrulama desteği
- **Otomatik Keşif**: Mattermost veritabanı bilgilerini `config.json` dosyasından otomatik okur
- **Esnek Matrix Kimlik Doğrulama**: Kullanıcı adı/şifre ile giriş veya mevcut admin token kullanımı
- **Güzel TUI**: Bubble Tea ile geliştirilmiş, stilli menülere sahip etkileşimli terminal arayüzü
- **Çoklu Dil Desteği**: İngilizce (varsayılan) ve Türkçe arayüz
- **Detaylı Bağlantı Testleri**: Sorunları tam olarak belirlemek için adım adım bağlantı tanılama
- **Devam Ettirilebilir**: Duraklatılıp devam ettirilebilen kontrol noktası tabanlı taşıma
- **Eşleme Dosyaları**: Mattermost → Matrix varlık ilişkilerini izlemek için eşleme dosyaları oluşturur

## Kurulum

```bash
go install github.com/aligundogdu/matrixmigrate/cmd/matrixmigrate@latest
```

Veya kaynaktan derleyin:

```bash
git clone https://github.com/aligundogdu/matrixmigrate.git
cd matrixmigrate
go build -o matrixmigrate ./cmd/matrixmigrate
```

## Yapılandırma

1. Örnek yapılandırmayı kopyalayın:
   ```bash
   cp config.example.yaml config.yaml
   ```

2. `config.yaml` dosyasını sunucu bilgilerinizle düzenleyin:

### SSH Anahtar Kimlik Doğrulaması (Önerilen)

```yaml
mattermost:
  ssh:
    host: "mattermost.example.com"
    user: "admin"
    key_path: "~/.ssh/id_rsa"
  config_path: "/opt/mattermost/config/config.json"

matrix:
  ssh:
    host: "matrix.example.com"
    user: "admin"
    key_path: "~/.ssh/id_rsa"
  auth:
    username: "admin"
    password_env: "MATRIX_ADMIN_PASSWORD"
  homeserver: "example.com"
```

### SSH Şifre Kimlik Doğrulaması

```yaml
mattermost:
  ssh:
    host: "mattermost.example.com"
    user: "root"
    password_env: "MM_SSH_PASSWORD"  # Ortam değişkeni kullanır
  config_path: "/opt/mattermost/config/config.json"

matrix:
  ssh:
    host: "matrix.example.com"
    user: "root"
    password_env: "MX_SSH_PASSWORD"
  auth:
    username: "admin"
    password_env: "MATRIX_ADMIN_PASSWORD"
  homeserver: "example.com"
```

3. Ortam değişkenlerini ayarlayın:
   ```bash
   # SSH şifre kimlik doğrulaması için
   export MM_SSH_PASSWORD="mattermost-ssh-sifreniz"
   export MX_SSH_PASSWORD="matrix-ssh-sifreniz"
   
   # Matrix admin girişi için
   export MATRIX_ADMIN_PASSWORD="admin-sifreniz"
   ```

### Nasıl Çalışır

**Mattermost**: Araç SSH ile bağlanır ve veritabanı bilgilerini almak için `/opt/mattermost/config/config.json` dosyasını okur. Manuel veritabanı yapılandırmasına gerek yok!

**Matrix**: Araç erişim token'ı almak için kullanıcı adı/şifre ile giriş yapar. Alternatif olarak, `MATRIX_ADMIN_TOKEN` ortam değişkeni ile mevcut bir admin token sağlayabilirsiniz.

## Kullanım

### Etkileşimli Mod (TUI)

```bash
# Varsayılan dil (İngilizce) ile başlat
./matrixmigrate

# Türkçe arayüz ile başlat
./matrixmigrate --lang tr
```

### Toplu İşlem Modu

```bash
# Belirli adımları çalıştır
./matrixmigrate export assets
./matrixmigrate import assets
./matrixmigrate export memberships
./matrixmigrate import memberships

# Belirli config ile çalıştır
./matrixmigrate --config ./config.yaml export assets
```

### Bağlantı Testi

Bağlantı testi detaylı adım adım tanılama sağlar:

```bash
# Tüm bağlantıları test et
./matrixmigrate test all

# Ayrı ayrı bağlantıları test et
./matrixmigrate test mattermost
./matrixmigrate test matrix
```

**Test Çıktısı Örneği:**
```
📋 Yapılandırma
   ✓ Yapılandırma dosyası yüklendi (config.yaml bulundu ve ayrıştırıldı)
   ✓ Veri dizinleri erişilebilir (Assets: ./data/assets, Mappings: ./data/mappings)

🗄️ Mattermost
   ✓ SSH yapılandırması ($MM_SSH_PASSWORD ile şifre doğrulama)
   ✓ SSH bağlantısı (root@mattermost.example.com:22)
   ✓ Mattermost config.json (/opt/mattermost/config/config.json)
   ✓ Veritabanı bağlantısı (150 kullanıcı, 12 takım, 87 kanal)

🔷 Matrix
   ✓ SSH yapılandırması (Anahtar: ~/.ssh/id_rsa)
   ✓ SSH bağlantısı (admin@matrix.example.com:22)
   ✓ API kimlik doğrulama ($MATRIX_ADMIN_PASSWORD ile admin olarak giriş)
   ✓ API bağlantısı (Homeserver: example.com)

✓ Tüm bağlantı testleri başarılı!
```

### Durum Kontrolü

```bash
./matrixmigrate status
```

## Taşıma Adımları

| Adım | Komut | Açıklama |
|------|-------|----------|
| 1a | `export assets` | Mattermost'tan kullanıcıları, takımları, kanalları dışa aktar |
| 1b | `import assets` | Matrix'te kullanıcıları, space'leri, odaları oluştur |
| 2a | `export memberships` | Mattermost'tan takım/kanal üyeliklerini dışa aktar |
| 2b | `import memberships` | Matrix'te üyelikleri uygula |

## Mimari

```
┌─────────────────────────────────────────────────────────────┐
│                    Yerel Makine                             │
│  ┌─────────────┐  ┌──────────┐  ┌─────────────────────────┐│
│  │ MatrixMigrate│  │ Config   │  │ Veri Deposu             ││
│  │     CLI     │──│  YAML    │  │ - assets/*.json.gz      ││
│  └──────┬──────┘  └──────────┘  │ - mappings/*.json       ││
│         │                       │ - state.json            ││
│         │                       └─────────────────────────┘│
└─────────┼───────────────────────────────────────────────────┘
          │
    ┌─────┴─────┐
    │           │
    ▼           ▼
┌────────┐  ┌────────┐
│Mattermost│  │ Matrix │
│SSH (anahtar/│ │SSH (anahtar/│
│şifre)   │  │şifre)  │
│    ↓     │  │   ↓    │
│config.json│  │  API   │
│    ↓     │  │   ↓    │
│PostgreSQL│  │Giriş/Token
└────────┘  └────────┘
```

## Mattermost → Matrix Eşlemesi

| Mattermost | Matrix |
|------------|--------|
| Team | Space |
| Channel | Room |
| User | User |
| Team Membership | Space Membership |
| Channel Membership | Room Membership |

## Ortam Değişkenleri

| Değişken | Açıklama | Zorunlu |
|----------|----------|---------|
| `MATRIX_ADMIN_PASSWORD` | Giriş için Matrix admin şifresi | Evet (auth kullanılıyorsa) |
| `MATRIX_ADMIN_TOKEN` | Alternatif: mevcut admin token | Hayır |
| `MM_SSH_PASSWORD` | Mattermost SSH şifresi | Hayır (anahtar kullanılıyorsa) |
| `MX_SSH_PASSWORD` | Matrix SSH şifresi | Hayır (anahtar kullanılıyorsa) |
| `SSH_KEY_PASSPHRASE` | SSH anahtar parolası (şifreli ise) | Hayır |

## Sorun Giderme

Bağlantının tam olarak nerede başarısız olduğunu belirlemek için `./matrixmigrate test all` kullanın.

### SSH Bağlantısı Başarısız
- Anahtar doğrulama için: SSH anahtarının düzgün yapılandırıldığından ve doğru izinlere sahip olduğundan emin olun
- Şifre doğrulama için: Şifre ortam değişkeninin ayarlandığını kontrol edin
- SSH portunun doğru olduğunu doğrulayın (varsayılan: 22)

### Mattermost Config Bulunamadı
- config.yaml dosyanızdaki `config_path` değerini kontrol edin
- Farklı yolları deneyin: `/opt/mattermost/config/config.json`, `/opt/mattermost/config.json`
- SSH kullanıcısının dosyaya okuma erişimi olduğundan emin olun

### Matrix Girişi Başarısız
- Admin kullanıcı adı ve şifresini doğrulayın
- Matrix homeserver'ın şifre girişini destekleyip desteklemediğini kontrol edin
- Kullanıcının admin yetkilerine sahip olduğundan emin olun

### Veritabanı Bağlantısı Başarısız
- Araç, kimlik bilgilerini Mattermost'un config.json dosyasından otomatik olarak okur
- PostgreSQL'in çalıştığından ve Mattermost sunucusunda localhost'tan erişilebilir olduğundan emin olun

## Lisans

MIT Lisansı
