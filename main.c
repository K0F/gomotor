#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <math.h>
#include <unistd.h>
#include <fcntl.h>
#include <termios.h>
#include <signal.h>
#include <getopt.h>
#include <ctype.h>

#define MAX_LINE_LENGTH 1024
#define DEFAULT_BAUD B115200

// V-plotter Configuration Constants
typedef struct {
    double motor_ax;
    double motor_ay;
    double motor_bx;
    double motor_by;
    double steps_per_mm_a;
    double steps_per_mm_b;
    double center_len_a;
    double center_len_b;
    double segment_size;
    int speed;
    int invert_y;
    double scale;
    double offset_x;
    double offset_y;
} PlotterConfig;

// Global serial fd and interruption flag for safe cleanup
int serial_fd = -1;
volatile sig_atomic_t keep_running = 1;

void handle_sigint(int sig) {
    (void)sig;
    keep_running = 0;
    if (serial_fd != -1) {
        // Send pen up command on emergency abort
        write(serial_fd, "P0\n", 3);
        close(serial_fd);
    }
    fprintf(stderr, "\n[!] Operation aborted by user. Exiting safely...\n");
    exit(0);
}

int configure_serial(const char *port_name, speed_t baud) {
    int fd = open(port_name, O_RDWR | O_NOCTTY | O_SYNC);
    if (fd < 0) {
        perror("Error opening serial port");
        return -1;
    }

    struct termios tty;
    if (tcgetattr(fd, &tty) != 0) {
        perror("Error from tcgetattr");
        close(fd);
        return -1;
    }

    cfsetospeed(&tty, baud);
    cfsetispeed(&tty, baud);

    tty.c_cflag = (tty.c_cflag & ~CSIZE) | CS8;     // 8-bit characters
    tty.c_iflag &= ~IGNBRK;                         // disable break processing
    tty.c_lflag = 0;                                // no signaling chars, no echo, no canonical processing
    tty.c_oflag = 0;                                // no remapping, no delays
    tty.c_cc[VMIN]  = 0;                            // read doesn't block
    tty.c_cc[VTIME] = 5;                            // 0.5 seconds read timeout

    tty.c_cflag |= (CLOCAL | CREAD);                // ignore modem controls, enable reading
    tty.c_cflag &= ~(PARENB | PARODD);              // shut off parity
    tty.c_cflag &= ~CSTOPB;
    tty.c_cflag &= ~CRTSCTS;

    if (tcsetattr(fd, TCSANOW, &tty) != 0) {
        perror("Error from tcsetattr");
        close(fd);
        return -1;
    }

    return fd;
}

int wait_for_ok(int fd) {
    char buf[32];
    char line_buf[128];
    int line_idx = 0;
    
    while (keep_running) {
        int n = read(fd, buf, 1);
        if (n > 0) {
            char c = buf[0];
            if (c == '\n') {
                line_buf[line_idx] = '\0';
                if (strncmp(line_buf, "ok", 2) == 0) {
                    return 1;
                }
                line_idx = 0;
            } else if (c != '\r' && line_idx < (int)sizeof(line_buf) - 1) {
                line_buf[line_idx++] = c;
            }
        }
    }
    return 0;
}

int send_command(int fd, const char *cmd) {
    if (fd < 0) return 0;
    dprintf(fd, "%s\n", cmd);
    return wait_for_ok(fd);
}

// Kinematics calculations
void compute_string_lengths(double x, double y, const PlotterConfig *cfg, double *len_a, double *len_b) {
    double dx_a = x - cfg->motor_ax;
    double dy_a = y - cfg->motor_ay;
    *len_a = sqrt(dx_a * dx_a + dy_a * dy_a);

    double dx_b = x - cfg->motor_bx;
    double dy_b = y - cfg->motor_by;
    *len_b = sqrt(dx_b * dx_b + dy_b * dy_b);
}

void moveTo(int fd, double x, double y, const PlotterConfig *cfg) {
    if (!keep_running) return;

    double la, lb;
    compute_string_lengths(x, y, cfg, &la, &lb);

    long steps_a = lround((la - cfg->center_len_a) * cfg->steps_per_mm_a);
    long steps_b = lround((lb - cfg->center_len_b) * cfg->steps_per_mm_b);

    char cmd[128];
    snprintf(cmd, sizeof(cmd), "X%ldY%ld", steps_a, steps_b);
    send_command(fd, cmd);
}

void draw_line(int fd, double x0, double y0, double x1, double y1, const PlotterConfig *cfg) {
    double dx = x1 - x0;
    double dy = y1 - y0;
    double dist = sqrt(dx * dx + dy * dy);
    int segments = (int)ceil(dist / cfg->segment_size);
    if (segments < 1) segments = 1;

    for (int i = 1; i <= segments; i++) {
        if (!keep_running) break;
        double t = (double)i / (double)segments;
        double nx = x0 + dx * t;
        double ny = y0 + dy * t;
        moveTo(fd, nx, ny, cfg);
    }
}

// Simplified SVG Path Processor & Parser (handles M, L commands)
void parse_and_draw_svg(int fd, const char *filepath, const PlotterConfig *cfg) {
    FILE *f = fopen(filepath, "r");
    if (!f) {
        perror("Error opening SVG file");
        return;
    }

    char line[MAX_LINE_LENGTH];
    double current_x = 0.0, current_y = 0.0;
    double start_x = 0.0, start_y = 0.0;
    int pen_down = 0;

    // Set initial speed
    char speed_cmd[32];
    snprintf(speed_cmd, sizeof(speed_cmd), "S%d", cfg->speed);
    send_command(fd, speed_cmd);

    while (fgets(line, sizeof(line), f) && keep_running) {
        // Look for path 'd' attribute or basic geometric primitives in SVG
        char *d_attr = strstr(line, "d=\"");
        if (d_attr) {
            d_attr += 3;
            char *end_quote = strchr(d_attr, '"');
            if (end_quote) {
                *end_quote = '\0';
                
                // Parse path commands
                char cmd = '\0';
                char *ptr = d_attr;
                while (*ptr) {
                    while (*ptr && isspace(*ptr)) ptr++;
                    if (!*ptr) break;

                    if (isalpha(*ptr)) {
                        cmd = *ptr;
                        ptr++;
                    }

                    while (*ptr && isspace(*ptr)) ptr++;
                    if (!*ptr) break;

                    if (cmd == 'M' || cmd == 'm' || cmd == 'L' || cmd == 'l') {
                        double x = 0, y = 0;
                        if (sscanf(ptr, "%lf,%lf", &x, &y) == 2 || sscanf(ptr, "%lf %lf", &x, &y) == 2) {
                            if (cmd == 'm' || cmd == 'l') {
                                x += current_x;
                                y += current_y;
                            }

                            // Apply scaling and offsets
                            double tx = x * cfg->scale + cfg->offset_x;
                            double ty = (cfg->invert_y ? -y : y) * cfg->scale + cfg->offset_y;

                            if (cmd == 'M' || cmd == 'm') {
                                if (pen_down) {
                                    send_command(fd, "P0"); // Pen Up
                                    pen_down = 0;
                                }
                                moveTo(fd, tx, ty, cfg);
                                start_x = tx;
                                start_y = ty;
                            } else if (cmd == 'L' || cmd == 'l') {
                                if (!pen_down) {
                                    send_command(fd, "P1"); // Pen Down
                                    pen_down = 1;
                                }
                                draw_line(fd, current_x, current_y, tx, ty, cfg);
                            }
                            current_x = tx;
                            current_y = ty;

                            // Advance pointer past coordinates
                            while (*ptr && (*ptr != ' ' && *ptr != ',' && !isalpha(*ptr))) ptr++;
                            while (*ptr && (*ptr == ' ' || *ptr == ',')) ptr++;
                            while (*ptr && (*ptr != ' ' && *ptr != ',' && !isalpha(*ptr))) ptr++;
                        } else {
                            ptr++;
                        }
                    } else if (cmd == 'Z' || cmd == 'z') {
                        if (pen_down) {
                            draw_line(fd, current_x, current_y, start_x, start_y, cfg);
                            current_x = start_x;
                            current_y = start_y;
                        }
                        cmd = '\0';
                    } else {
                        ptr++;
                    }
                }
            }
        }
    }

    if (pen_down) {
        send_command(fd, "P0"); // Ensure pen is up at the end
    }

    fclose(f);
}

void print_usage(const char *prog) {
    printf("Usage: %s [options]\n", prog);
    printf("  -f, --file <path>      Path to input SVG file (Required)\n");
    printf("  -p, --port <port>      Serial port device (default: /dev/ttyUSB0)\n");
    printf("  -s, --scale <val>      Uniform scale factor (default: 1.0)\n");
    printf("  -x, --offx <val>       X offset in mm (default: 0.0)\n");
    printf("  -y, --offy <val>       Y offset in mm (default: 0.0)\n");
    printf("  -v, --speed <val>      Motor movement speed (default: 300)\n");
    printf("  -h, --help             Display this help message\n");
}

int main(int argc, char *argv[]) {
    signal(SIGINT, handle_sigint);

    PlotterConfig cfg = {
        .motor_ax = 0.0,
        .motor_ay = 0.0,
        .motor_bx = 2000.0, // 2 meters between motors
        .motor_by = 0.0,
        .steps_per_mm_a = 80.0,
        .steps_per_mm_b = 80.0,
        .center_len_a = 1000.0,
        .center_len_b = 1000.0,
        .segment_size = 0.5,
        .speed = 300,
        .invert_y = 1,
        .scale = 1.0,
        .offset_x = 500.0,
        .offset_y = 500.0
    };

    char svg_file[256] = "";
    char port_name[256] = "/dev/ttyUSB0";

    static struct option long_options[] = {
        {"file",  required_argument, 0, 'f'},
        {"port",  required_argument, 0, 'p'},
        {"scale", required_argument, 0, 's'},
        {"offx",  required_argument, 0, 'x'},
        {"offy",  required_argument, 0, 'y'},
        {"speed", required_argument, 0, 'v'},
        {"help",  no_argument,       0, 'h'},
        {0, 0, 0, 0}
    };

    int opt;
    while ((opt = getopt_long(argc, argv, "f:p:s:x:y:v:h", long_options, NULL)) != -1) {
        switch (opt) {
            case 'f':
                strncpy(svg_file, optarg, sizeof(svg_file) - 1);
                break;
            case 'p':
                strncpy(port_name, optarg, sizeof(port_name) - 1);
                break;
            case 's':
                cfg.scale = atof(optarg);
                break;
            case 'x':
                cfg.offset_x = atof(optarg);
                break;
            case 'y':
                cfg.offset_y = atof(optarg);
                break;
            case 'v':
                cfg.speed = atoi(optarg);
                break;
            case 'h':
            default:
                print_usage(argv[0]);
                exit(EXIT_SUCCESS);
        }
    }

    if (strlen(svg_file) == 0) {
        fprintf(stderr, "Error: Input SVG file is required (-f / --file).\n\n");
        print_usage(argv[0]);
        exit(EXIT_FAILURE);
    }

    printf("[*] Connecting to serial port %s at 115200 baud...\n", port_name);
    serial_fd = configure_serial(port_name, DEFAULT_BAUD);
    if (serial_fd < 0) {
        fprintf(stderr, "Failed to initialize serial communication.\n");
        exit(EXIT_FAILURE);
    }

    // Wait a moment for Arduino reset after DTR toggle
    usleep(2000000);

    printf("[*] Parsing and streaming vector graphic: %s\n", svg_file);
    parse_and_draw_svg(serial_fd, svg_file, &cfg);

    // Return pen up and close
    send_command(serial_fd, "P0");
    close(serial_fd);
    printf("[*] Plotting completed successfully.\n");

    return 0;
}
