#ifndef XLUNCH_NATIVE_H
#define XLUNCH_NATIVE_H

#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <X11/Xatom.h>
#include <X11/keysym.h>
#include <X11/Xft/Xft.h>
#include <Imlib2.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <math.h>
#include <time.h>
#include <unistd.h>

typedef struct {
    Display *display;
    Window window;
    Window root;
    int screen;
    GC gc;
    XftDraw *xft_draw;
    XftFont *font;
    XftColor text_color;
    XftColor highlight_color;
    XftColor background_color;
    Colormap colormap;
    Visual *visual;
    int width;
    int height;
    int icon_size;
    int cols;
    int rows;
    int cell_width;
    int cell_height;
    int padding;
    int num_entries;  // Number of actual entries for bounds checking
    Imlib_Image background_image;
    XVisualInfo vinfo;
    XSetWindowAttributes attr;
    
    // Button tracking for hover effects and clicks
    int settings_button_hovered;
    int logo_button_hovered;
    int settings_x, settings_y, settings_w, settings_h;
    int logo_x, logo_y, logo_w, logo_h;
    
    // Scroll tracking (like xlunch)
    int scrolled_past;        // Number of rows scrolled past (like xlunch)
    int noscroll;            // Disable scrolling if set
    int entries_count;       // Total number of visible entries
    unsigned long scrollbar_color;      // Scrollbar background color
    unsigned long scrollindicator_color; // Scrollbar indicator color
    int scroll_debounce_counter; // For scroll debouncing
    
    // Double buffering support to prevent flicker
    Imlib_Image render_buffer;   // Off-screen render buffer
    int dirty;                   // Flag to track if redraw is needed
} XLunchNative;

// Initialize native xlunch - mimics xlunch.c init exactly
XLunchNative* xlunch_native_init(int width, int height, int icon_size);

// Show the window
void xlunch_native_show(XLunchNative *xl);

// Set xlunch theme colors exactly like original
void xlunch_native_set_theme_colors(XLunchNative *xl, const char *theme);

// Draw text at position with xlunch-style formatting
void xlunch_native_draw_text(XLunchNative *xl, int x, int y, const char *text, int highlighted);

// Draw a rectangle (for highlighting)
void xlunch_native_draw_rect(XLunchNative *xl, int x, int y, int width, int height, int filled);

// Clear the window and draw background like xlunch
void xlunch_native_clear(XLunchNative *xl);

// Present the render buffer to screen (double buffering)
void xlunch_native_present(XLunchNative *xl);

// Draw background like xlunch
void xlunch_native_draw_background(XLunchNative *xl);

// Draw background image (or use default dark background)
void xlunch_native_draw_background_image(XLunchNative *xl, const char *image_path);

// Draw icon using Imlib2 like xlunch does
void xlunch_native_draw_icon(XLunchNative *xl, int x, int y, int width, int height, const char *icon_path);

// Show app details with action buttons
void xlunch_native_show_app_details(XLunchNative *xl, const char *app_name, const char *description, const char *status, int show_install, int show_uninstall);

// Handle app details input (returns: 1=install, 2=uninstall, 0=back, -1=continue)
int xlunch_native_handle_app_details_input(XLunchNative *xl);

// Process events (returns 1 for continue, 0 for quit, -1 for error)
int xlunch_native_handle_events(XLunchNative *xl, int *selected_entry);

// Draw rounded rectangle (simulate rounded corners)
void xlunch_native_draw_rounded_rect(XLunchNative *xl, int x, int y, int width, int height, int radius, unsigned long color);

// Draw button with hover effect
void xlunch_native_draw_button_with_hover(XLunchNative *xl, int x, int y, int width, int height, const char *icon_path, int hovered);

// Cleanup
void xlunch_native_cleanup(XLunchNative *xl);

// Check if mouse is over a button (returns 1 for settings, 2 for logo, 0 for none)
int xlunch_native_check_button_hover(XLunchNative *xl, int mouse_x, int mouse_y);

// Handle button hover effects
void xlunch_native_handle_hover(XLunchNative *xl, int mouse_x, int mouse_y);

// Set scroll level (like xlunch set_scroll_level) - returns 1 if changed, 0 if no change
int xlunch_native_set_scroll_level(XLunchNative *xl, int new_scroll);

// Draw scrollbar (like xlunch scrollbar drawing)
void xlunch_native_draw_scrollbar(XLunchNative *xl);

// Calculate entry position with scroll offset
void xlunch_native_calculate_entry_position(XLunchNative *xl, int entry_index, int *x, int *y);

#endif // XLUNCH_NATIVE_H 