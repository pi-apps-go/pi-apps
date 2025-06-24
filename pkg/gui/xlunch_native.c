#include "xlunch_native.h"

// Root image capture functions - exactly like xlunch
static Pixmap GetRootPixmap(Display* display, Window *root) {
    Pixmap currentRootPixmap = None;
    Atom act_type;
    int act_format;
    unsigned long nitems, bytes_after;
    unsigned char *data = NULL;
    Atom _XROOTPMAP_ID;

    _XROOTPMAP_ID = XInternAtom(display, "_XROOTPMAP_ID", False);

    if (XGetWindowProperty(display, *root, _XROOTPMAP_ID, 0, 1, False,
                          XA_PIXMAP, &act_type, &act_format, &nitems, &bytes_after,
                          &data) == Success) {
        if (data) {
            currentRootPixmap = *((Pixmap *) data);
            XFree(data);
            printf("Found root pixmap: %lu\n", currentRootPixmap);
        } else {
            printf("XGetWindowProperty succeeded but no data returned\n");
        }
    } else {
        printf("XGetWindowProperty failed for _XROOTPMAP_ID\n");
        
        // Fallback: try _XROOTMAP_ID (some window managers use this)
        Atom _XROOTMAP_ID = XInternAtom(display, "_XROOTMAP_ID", False);
        if (XGetWindowProperty(display, *root, _XROOTMAP_ID, 0, 1, False,
                              XA_PIXMAP, &act_type, &act_format, &nitems, &bytes_after,
                              &data) == Success) {
            if (data) {
                currentRootPixmap = *((Pixmap *) data);
                XFree(data);
                printf("Found root pixmap via _XROOTMAP_ID fallback: %lu\n", currentRootPixmap);
            }
        }
    }

    if (currentRootPixmap == None) {
        printf("No root pixmap found - desktop background capture not available\n");
    }

    return currentRootPixmap;
}

// Try alternative X11 methods for desktop capture
static int xl_capture_desktop_alternative(XLunchNative *xl, DATA32 *direct) {
    XWindowAttributes root_attrs;
    XGetWindowAttributes(xl->display, xl->root, &root_attrs);
    
    // Method 1: Try XShmGetImage if available (faster, might bypass some restrictions)
    XImage* img = NULL;
    
    // Method 2: Try different plane masks
    unsigned long plane_masks[] = {
        AllPlanes,
        0xFFFFFF,  // RGB only
        ~0UL,      // All bits
        0x00FFFFFF // 24-bit mask
    };
    
    for (int i = 0; i < 4 && !img; i++) {
        img = XGetImage(xl->display, xl->root, 0, 0, 
                       root_attrs.width, root_attrs.height, 
                       plane_masks[i], ZPixmap);
        if (img) {
            printf("Desktop capture succeeded with plane mask %d\n", i);
            break;
        }
    }
    
    if (!img) {
        // Method 3: Try capturing a smaller area first to test
        img = XGetImage(xl->display, xl->root, 0, 0, 100, 100, AllPlanes, ZPixmap);
        if (img) {
            XDestroyImage(img);
            // If small capture works, try full screen again
            img = XGetImage(xl->display, xl->root, 0, 0, 
                           root_attrs.width, root_attrs.height, AllPlanes, ZPixmap);
            printf("Desktop capture succeeded after small area test\n");
        }
    }
    
    if (!img) {
        printf("All X11 desktop capture methods failed\n");
        return 0;
    }
    
    // Calculate window position (we'll need this for cropping)
    int center_x = (root_attrs.width - xl->width) / 2;
    int center_y = (root_attrs.height - xl->height) / 2;
    
    // Copy the relevant portion of the desktop
    for (int y = 0; y < xl->height; y++) {
        for (int x = 0; x < xl->width; x++) {
            int src_x = center_x + x;
            int src_y = center_y + y;
            
            if (src_x >= 0 && src_y >= 0 && src_x < img->width && src_y < img->height) {
                unsigned long pixel = XGetPixel(img, src_x, src_y);
                direct[y * xl->width + x] = 0xFFFFFF & pixel;
            } else {
                direct[y * xl->width + x] = 0xFF000000; // Black fallback
            }
        }
    }
    
    XDestroyImage(img);
    printf("Successfully captured desktop using alternative X11 method\n");
    return 1;
}

static int xl_get_root_image_to_imlib_data(XLunchNative *xl, DATA32 *direct) {
    if (!xl || !direct) {
        return 0;
    }
    
    XWindowAttributes attrs;
    Pixmap bg;
    XImage* img;

    // Get root window attributes to get full screen dimensions
    XGetWindowAttributes(xl->display, xl->root, &attrs);

    bg = GetRootPixmap(xl->display, &xl->root);
    if (bg && bg != None) {
        // Use background pixmap if available (traditional method)
        img = XGetImage(xl->display, bg, 0, 0, attrs.width, attrs.height, ~0, ZPixmap);
        XFreePixmap(xl->display, bg);
        printf("Using background pixmap method\n");
    } else {
        printf("GetRootPixmap failed - trying direct root window capture\n");
        // Fallback: capture root window directly (works with modern compositors)
        img = XGetImage(xl->display, xl->root, 0, 0, attrs.width, attrs.height, AllPlanes, ZPixmap);
        printf("Using direct root window capture method\n");
    }

    if (!img) {
        printf("XGetImage failed - trying alternative X11 methods\n");
        return xl_capture_desktop_alternative(xl, direct);
    }
    
    // Get window position for cropping the captured image
    Window dummy;
    int win_x, win_y;
    unsigned int dummy_uint;
    XGetGeometry(xl->display, xl->window, &dummy, &win_x, &win_y, 
                 &dummy_uint, &dummy_uint, &dummy_uint, &dummy_uint);
    
    // Convert window coordinates to root coordinates
    Window child;
    XTranslateCoordinates(xl->display, xl->window, xl->root, 0, 0, &win_x, &win_y, &child);
    
    unsigned long pixel;
    int x, y;

    // Copy the captured image data, cropping to window area like xlunch
    for (y = 0; y < xl->height && (y + win_y) < img->height; y++) {
        for (x = 0; x < xl->width && (x + win_x) < img->width; x++) {
            // Get pixel from captured background at window position
            int src_x = x + win_x;
            int src_y = y + win_y;
            if (src_x >= 0 && src_y >= 0 && src_x < img->width && src_y < img->height) {
                pixel = XGetPixel(img, src_x, src_y);
                direct[y * xl->width + x] = 0xffffffff & pixel;
            } else {
                // Fill with black if outside bounds
                direct[y * xl->width + x] = 0xff000000;
            }
        }
    }

    XDestroyImage(img);
    printf("Successfully captured root background (%dx%d at %d,%d)\n", 
           xl->width, xl->height, win_x, win_y);
    return 1;
}

// Initialize native xlunch - mimics xlunch.c init exactly
XLunchNative* xlunch_native_init(int width, int height, int icon_size) {
    XLunchNative *xl = malloc(sizeof(XLunchNative));
    if (!xl) return NULL;

    // Initialize all pointers to NULL for safe cleanup
    xl->display = NULL;
    xl->window = 0;
    xl->xft_draw = NULL;
    xl->font = NULL;
    xl->background_image = NULL;
    xl->render_buffer = NULL;
    xl->dirty = 1; // Start with dirty flag set

    xl->display = XOpenDisplay(NULL);
    if (!xl->display) {
        fprintf(stderr, "Failed to open X11 display\n");
        free(xl);
        return NULL;
    }

    xl->screen = DefaultScreen(xl->display);
    xl->root = RootWindow(xl->display, xl->screen);

    // Use xlunch's exact visual setup - prefer 24-bit for pseudo-transparency
    if (!XMatchVisualInfo(xl->display, xl->screen, 24, TrueColor, &xl->vinfo)) {
        if (!XMatchVisualInfo(xl->display, xl->screen, 32, TrueColor, &xl->vinfo)) {
           if (!XMatchVisualInfo(xl->display, xl->screen, 16, DirectColor, &xl->vinfo)) {
              XMatchVisualInfo(xl->display, xl->screen, 8, PseudoColor, &xl->vinfo);
           }
        }
    }

    // Use vinfo.visual consistently like xlunch does
    xl->visual = xl->vinfo.visual;
    
    // Create colormap exactly like xlunch  
    xl->attr.colormap = XCreateColormap(xl->display, xl->root, xl->vinfo.visual, AllocNone);
    xl->attr.border_pixel = 0;
    xl->attr.background_pixel = BlackPixel(xl->display, xl->screen); // Use solid background for pseudo-transparency
    xl->attr.override_redirect = False;
    xl->attr.backing_store = Always;

    xl->colormap = xl->attr.colormap;

    xl->width = width;
    xl->height = height;
    xl->icon_size = icon_size;
    xl->padding = 20;

    // Calculate grid layout like xlunch
    xl->cell_width = icon_size + xl->padding * 2;
    xl->cell_height = icon_size + xl->padding * 2 + 20; // space for text
    xl->cols = (width - xl->padding) / xl->cell_width;
    xl->rows = (height - xl->padding) / xl->cell_height;

    // Create window with proper visual for transparency - exactly like xlunch
    xl->window = XCreateWindow(xl->display, xl->root,
                              (DisplayWidth(xl->display, xl->screen) - width) / 2,
                              (DisplayHeight(xl->display, xl->screen) - height) / 2,
                              width, height, 0, xl->vinfo.depth, InputOutput,
                              xl->vinfo.visual, CWColormap | CWBorderPixel | CWBackPixel | CWOverrideRedirect | CWBackingStore, &xl->attr);

    // Set window properties like xlunch
    XStoreName(xl->display, xl->window, "Pi-Apps Go: Raspberry Pi and cross-platform app store");
    
    // Set application name
    XSetClassHint(xl->display, xl->window, &(XClassHint){"pi-apps", "Pi-Apps"});
    
    // No window opacity needed - xlunch uses pseudo-transparency with background capture

    // Create graphics context
    xl->gc = XCreateGC(xl->display, xl->window, 0, NULL);

    // Initialize Xft for text rendering with consistent visual
    xl->xft_draw = XftDrawCreate(xl->display, xl->window, xl->vinfo.visual, xl->colormap);
    xl->font = XftFontOpenName(xl->display, xl->screen, "DejaVuSans-10");
    if (!xl->font) {
        xl->font = XftFontOpenName(xl->display, xl->screen, "Sans-10");
    }

    // Initialize colors exactly like xlunch themes
    XftColorAllocName(xl->display, xl->vinfo.visual, xl->colormap, "#ffffff", &xl->text_color);
    XftColorAllocName(xl->display, xl->vinfo.visual, xl->colormap, "#6060ff", &xl->highlight_color);
    XftColorAllocName(xl->display, xl->vinfo.visual, xl->colormap, "#000000", &xl->background_color);

    // Set event mask like xlunch
    XSelectInput(xl->display, xl->window,
                ExposureMask | KeyPressMask | ButtonPressMask | ButtonReleaseMask |
                PointerMotionMask | EnterWindowMask | LeaveWindowMask);

    // Initialize button states and coordinates
    xl->settings_button_hovered = 0;
    xl->logo_button_hovered = 0;
    
    // Set button coordinates based on theme (will be updated in drawing functions)
    xl->settings_x = width - 150;
    xl->settings_y = 30;
    xl->settings_w = 140;
    xl->settings_h = 52;
    
    // Logo coordinates depend on theme - default for transparent theme
    xl->logo_x = 45;
    xl->logo_y = 10;
    xl->logo_w = 245;
    xl->logo_h = 100;

    // Initialize scroll state (like xlunch)
    xl->scrolled_past = 0;
    xl->noscroll = 0;
    xl->entries_count = 0;
    // Scrollbar colors (like xlunch defaults: white with alpha)
    xl->scrollbar_color = 0x3CFFFFFF;      // White with 60/255 alpha (~0x3C)
    xl->scrollindicator_color = 0x70FFFFFF; // White with 112/255 alpha (~0x70)
    xl->scroll_debounce_counter = 0; // Initialize scroll debouncing

    // Initialize Imlib2 exactly like xlunch - consistent visual usage
    imlib_set_cache_size(2048 * width);
    imlib_set_font_cache_size(512 * width);
    imlib_context_set_display(xl->display);
    imlib_context_set_visual(xl->vinfo.visual);
    imlib_context_set_colormap(xl->colormap);
    imlib_context_set_drawable(xl->window);
    imlib_context_set_dither(1);

    // Create render buffer for double buffering (prevents flicker)
    xl->render_buffer = imlib_create_image(width, height);
    if (xl->render_buffer) {
        printf("Created double buffer for flicker-free rendering\n");
    }

    // Create pseudo-transparent background exactly like xlunch
    xl->background_image = imlib_create_image(width, height);
    if (xl->background_image) {
        imlib_context_set_image(xl->background_image);
        
        // No alpha channel - xlunch uses solid pixels for pseudo-transparency
        imlib_image_set_has_alpha(0);
        
        // Try root image capture first (like xlunch does)
        DATA32 *direct = imlib_image_get_data();
        if (direct && xl_get_root_image_to_imlib_data(xl, direct)) {
            // Apply semi-transparent dark overlay exactly like xlunch
            // xlunch uses alpha blending with 0xA0 (160/255 = ~63% opacity)
            for (int i = 0; i < width * height; i++) {
                // Extract RGB from captured background
                int r = (direct[i] >> 16) & 0xFF;
                int g = (direct[i] >> 8) & 0xFF;
                int b = direct[i] & 0xFF;
                
                // Apply xlunch's semi-transparent black overlay (alpha = 160)
                r = (r * (255 - 160)) / 255;
                g = (g * (255 - 160)) / 255;
                b = (b * (255 - 160)) / 255;
                
                // Store as solid RGB (no alpha)
                direct[i] = 0xFF000000 | (r << 16) | (g << 8) | b;
            }
            imlib_image_put_back_data(direct);
            
            printf("Created pseudo-transparent background with real desktop capture\n");
        } else {
            // Modern compositor fallback: Create a clean, professional background
            // that looks like xlunch's dark theme without fake transparency
            printf("Root capture failed - using clean dark background like xlunch themes\n");
            
            // Use xlunch's actual dark theme colors (from xlunch source)
            // This matches what xlunch looks like with solid backgrounds
            imlib_context_set_color(46, 52, 64, 255); // xlunch dark theme base color
            imlib_image_fill_rectangle(0, 0, width, height);
            
            printf("Created clean dark background matching xlunch dark theme\n");
        }
    } else {
        printf("Failed to create background image\n");
    }

    return xl;
}

// Show the window
void xlunch_native_show(XLunchNative *xl) {
    if (!xl) return;
    XMapWindow(xl->display, xl->window);
    XFlush(xl->display);
}

// Set xlunch theme colors exactly like original
void xlunch_native_set_theme_colors(XLunchNative *xl, const char *theme) {
    if (!xl || !theme) return;
    
    // Free existing colors
    XftColorFree(xl->display, xl->vinfo.visual, xl->colormap, &xl->text_color);
    XftColorFree(xl->display, xl->vinfo.visual, xl->colormap, &xl->highlight_color);
    XftColorFree(xl->display, xl->vinfo.visual, xl->colormap, &xl->background_color);
    
    if (strcmp(theme, "light-3d") == 0) {
        // light mode: --tc 000000 (black text)
        XftColorAllocName(xl->display, xl->vinfo.visual, xl->colormap, "#000000", &xl->text_color);
        XftColorAllocName(xl->display, xl->vinfo.visual, xl->colormap, "#6060ff", &xl->highlight_color);
        XftColorAllocName(xl->display, xl->vinfo.visual, xl->colormap, "#e0e0e0", &xl->background_color);
    } else if (strcmp(theme, "dark-3d") == 0) {
        // dark mode 3d: --tc DCDDDE
        XftColorAllocName(xl->display, xl->vinfo.visual, xl->colormap, "#DCDDDE", &xl->text_color);
        XftColorAllocName(xl->display, xl->vinfo.visual, xl->colormap, "#ffffff", &xl->highlight_color);
        XftColorAllocName(xl->display, xl->vinfo.visual, xl->colormap, "#000000", &xl->background_color);
    } else {
        // transparent dark: --tc ffffffff (white text)
        XftColorAllocName(xl->display, xl->vinfo.visual, xl->colormap, "#ffffff", &xl->text_color);
        XftColorAllocName(xl->display, xl->vinfo.visual, xl->colormap, "#6060ff", &xl->highlight_color);
        XftColorAllocName(xl->display, xl->vinfo.visual, xl->colormap, "#000000", &xl->background_color);
    }
}

// Draw text at position with xlunch-style formatting
void xlunch_native_draw_text(XLunchNative *xl, int x, int y, const char *text, int highlighted) {
    if (!xl || !text) return;

    if (highlighted) {
        // Use highlight color for emphasized text
        XftDrawStringUtf8(xl->xft_draw, &xl->highlight_color, xl->font, x, y, (XftChar8 *)text, strlen(text));
    } else {
        // Use normal text color
        XftDrawStringUtf8(xl->xft_draw, &xl->text_color, xl->font, x, y, (XftChar8 *)text, strlen(text));
    }
}

// Draw a rectangle (for highlighting)
void xlunch_native_draw_rect(XLunchNative *xl, int x, int y, int width, int height, int filled) {
    if (!xl) return;

    if (filled) {
        XFillRectangle(xl->display, xl->window, xl->gc, x, y, width, height);
    } else {
        XDrawRectangle(xl->display, xl->window, xl->gc, x, y, width, height);
    }
}

// Clear the window and draw background like xlunch (with double buffering to prevent flicker)
void xlunch_native_clear(XLunchNative *xl) {
    if (!xl) return;
    
    // Use double buffering if available to prevent flicker
    if (xl->render_buffer) {
        // Draw to the off-screen buffer first
        imlib_context_set_image(xl->render_buffer);
        imlib_image_clear();
        
        if (xl->background_image) {
            // Copy background to render buffer
            imlib_blend_image_onto_image(xl->background_image, 1, 0, 0, xl->width, xl->height, 0, 0, xl->width, xl->height);
        } else {
            // Fill with dark background color
            imlib_context_set_color(46, 52, 64, 255);
            imlib_image_fill_rectangle(0, 0, xl->width, xl->height);
        }
    } else {
        // Fallback: direct rendering (may cause flicker)
        if (xl->background_image) {
            imlib_context_set_image(xl->background_image);
            imlib_context_set_drawable(xl->window);
            imlib_context_set_blend(1);
            imlib_render_image_on_drawable(0, 0);
        } else {
            XSetForeground(xl->display, xl->gc, 0x2e3440);
            XFillRectangle(xl->display, xl->window, xl->gc, 0, 0, xl->width, xl->height);
        }
    }
}

// Present the render buffer to screen (double buffering to prevent flicker)
void xlunch_native_present(XLunchNative *xl) {
    if (!xl || !xl->render_buffer) return;
    
    // Copy the render buffer to the window in one atomic operation
    imlib_context_set_image(xl->render_buffer);
    imlib_context_set_drawable(xl->window);
    imlib_context_set_blend(1);
    imlib_render_image_on_drawable(0, 0);
    
    // Flush to ensure the drawing is visible
    XFlush(xl->display);
}

// Draw background like xlunch
void xlunch_native_draw_background(XLunchNative *xl) {
    if (!xl) return;

    // Use the same background drawing as clear function
    xlunch_native_clear(xl);
}

// Draw background image (or use default dark background)
void xlunch_native_draw_background_image(XLunchNative *xl, const char *image_path) {
    if (!xl) return;

    if (image_path && strlen(image_path) > 0) {
        // Try to load actual background image file like xlunch
        Imlib_Image bg_image = imlib_load_image(image_path);
        if (bg_image) {
            imlib_context_set_image(bg_image);

            // Render background image scaled to window like xlunch
            imlib_context_set_drawable(xl->window);
            imlib_render_image_on_drawable_at_size(0, 0, xl->width, xl->height);

            imlib_free_image();
            return;
        }
    }

    // Fallback: simple dark background like xlunch default
    XSetForeground(xl->display, xl->gc, 0x2e3440); // xlunch default dark color
    XFillRectangle(xl->display, xl->window, xl->gc, 0, 0, xl->width, xl->height);
}

// Draw icon using Imlib2 like xlunch does
void xlunch_native_draw_icon(XLunchNative *xl, int x, int y, int width, int height, const char *icon_path) {
    if (!xl || !icon_path) return;

    // Try to load actual icon image like xlunch
    Imlib_Image icon = imlib_load_image(icon_path);
    if (icon) {
        imlib_context_set_image(icon);
        
        // Render icon to the render buffer if available, otherwise to window
        if (xl->render_buffer) {
            imlib_context_set_image(xl->render_buffer);
            imlib_context_set_blend(1);
        } else {
            imlib_context_set_drawable(xl->window);
            imlib_context_set_blend(1);
        }
        
        // Add small padding around icon (like original xlunch icon_padding)
        int icon_padding = 8;
        int actual_x = x + icon_padding;
        int actual_y = y + icon_padding;
        int actual_w = width - (icon_padding * 2);
        int actual_h = height - (icon_padding * 2);
        
        // Ensure we don't make icons too small
        if (actual_w > 16 && actual_h > 16) {
            if (xl->render_buffer) {
                imlib_context_set_image(icon);
                imlib_blend_image_onto_image(icon, 1, 0, 0, imlib_image_get_width(), imlib_image_get_height(), 
                                           actual_x, actual_y, actual_w, actual_h);
                imlib_context_set_image(xl->render_buffer);
            } else {
                imlib_render_image_on_drawable_at_size(actual_x, actual_y, actual_w, actual_h);
            }
        } else {
            // Fallback: render without padding if too small
            if (xl->render_buffer) {
                imlib_context_set_image(icon);
                imlib_blend_image_onto_image(icon, 1, 0, 0, imlib_image_get_width(), imlib_image_get_height(), 
                                           x, y, width, height);
                imlib_context_set_image(xl->render_buffer);
            } else {
                imlib_render_image_on_drawable_at_size(x, y, width, height);
            }
        }
        
        imlib_free_image();
    } else {
        // Fallback: draw colored rectangle based on category like xlunch
        unsigned long color;

        // Hash the path/title to get consistent colors per category
        unsigned long hash = 5381;
        for (const char *p = icon_path; *p; p++) {
            hash = ((hash << 5) + hash) + *p;
        }

        // Choose colors based on common pi-apps categories exactly like xlunch
        if (strstr(icon_path, "Games") || strstr(icon_path, "games")) {
            color = 0x9b59b6; // Purple
        } else if (strstr(icon_path, "Internet") || strstr(icon_path, "internet")) {
            color = 0x3498db; // Blue
        } else if (strstr(icon_path, "Multimedia") || strstr(icon_path, "multimedia")) {
            color = 0xe74c3c; // Red
        } else if (strstr(icon_path, "Office") || strstr(icon_path, "office")) {
            color = 0x2ecc71; // Green
        } else if (strstr(icon_path, "Programming") || strstr(icon_path, "programming")) {
            color = 0xf39c12; // Orange
        } else if (strstr(icon_path, "Engineering") || strstr(icon_path, "engineering")) {
            color = 0x34495e; // Dark blue-grey
        } else if (strstr(icon_path, "Appearance") || strstr(icon_path, "appearance")) {
            color = 0xe67e22; // Dark orange
        } else if (strstr(icon_path, "Tools") || strstr(icon_path, "tools")) {
            color = 0x95a5a6; // Grey
        } else {
            // Default app colors (avoid green tint) - use blue-based palette
            color = 0x3498db + (hash % 5) * 0x151515; // Blue variations without green
        }

        XSetForeground(xl->display, xl->gc, color);

        // Draw rounded rectangle like xlunch icons with proper padding
        int icon_padding = 8;
        int draw_x = x + icon_padding;
        int draw_y = y + icon_padding;
        int draw_w = width - (icon_padding * 2);
        int draw_h = height - (icon_padding * 2);
        
        int corner_radius = 6;
        XFillRectangle(xl->display, xl->window, xl->gc,
                      draw_x + corner_radius, draw_y,
                      draw_w - 2*corner_radius, draw_h);
        XFillRectangle(xl->display, xl->window, xl->gc,
                      draw_x, draw_y + corner_radius,
                      draw_w, draw_h - 2*corner_radius);

        // Draw corner arcs for rounded effect
        XFillArc(xl->display, xl->window, xl->gc,
                draw_x, draw_y,
                2*corner_radius, 2*corner_radius, 90*64, 90*64);
        XFillArc(xl->display, xl->window, xl->gc,
                draw_x + draw_w - 2*corner_radius, draw_y,
                2*corner_radius, 2*corner_radius, 0*64, 90*64);
        XFillArc(xl->display, xl->window, xl->gc,
                draw_x, draw_y + draw_h - 2*corner_radius,
                2*corner_radius, 2*corner_radius, 180*64, 90*64);
        XFillArc(xl->display, xl->window, xl->gc,
                draw_x + draw_w - 2*corner_radius, draw_y + draw_h - 2*corner_radius,
                2*corner_radius, 2*corner_radius, 270*64, 90*64);

        // Add highlight effect on top
        XSetForeground(xl->display, xl->gc, color + 0x333333);
        XDrawLine(xl->display, xl->window, xl->gc,
                 draw_x + corner_radius, draw_y + 1,
                 draw_x + draw_w - corner_radius, draw_y + 1);

        // Add shadow effect
        XSetForeground(xl->display, xl->gc, 0x000000);
        XDrawRectangle(xl->display, xl->window, xl->gc,
                      draw_x + 2, draw_y + 2,
                      draw_w - 1, draw_h - 1);
    }
}

// Process events (returns 1 for continue, 0 for quit with selection, -1 for error)
// Special return codes: -2 for settings button, -3 for logo button
int xlunch_native_handle_events(XLunchNative *xl, int *selected_entry) {
    if (!xl || !selected_entry) return -1;

    // Initialize selected_entry to invalid value
    *selected_entry = -1;

    XEvent event;
    if (XPending(xl->display) > 0) {
        XNextEvent(xl->display, &event);

        switch (event.type) {
        case KeyPress:
            if (event.xkey.keycode == 9) { // Escape key
                return 0; // Quit without selection
            }
            // Add keyboard scroll support (like xlunch)
            else if (event.xkey.keycode == 112) { // Page Up
                int scroll_changed = xlunch_native_set_scroll_level(xl, xl->scrolled_past - xl->rows);
                return scroll_changed ? 2 : 1; // Only redraw if scroll actually changed
            }
            else if (event.xkey.keycode == 117) { // Page Down
                int scroll_changed = xlunch_native_set_scroll_level(xl, xl->scrolled_past + xl->rows);
                return scroll_changed ? 2 : 1; // Only redraw if scroll actually changed
            }
            else if (event.xkey.keycode == 110) { // Home key
                int scroll_changed = xlunch_native_set_scroll_level(xl, 0);
                return scroll_changed ? 2 : 1; // Only redraw if scroll actually changed
            }
            else if (event.xkey.keycode == 115) { // End key
                int scroll_changed = xlunch_native_set_scroll_level(xl, xl->entries_count);
                return scroll_changed ? 2 : 1; // Only redraw if scroll actually changed
            }
            else if (event.xkey.keycode == 111) { // Up arrow
                int scroll_changed = xlunch_native_set_scroll_level(xl, xl->scrolled_past - 1);
                return scroll_changed ? 2 : 1; // Only redraw if scroll actually changed
            }
            else if (event.xkey.keycode == 116) { // Down arrow
                int scroll_changed = xlunch_native_set_scroll_level(xl, xl->scrolled_past + 1);
                return scroll_changed ? 2 : 1; // Only redraw if scroll actually changed
            }
            break;

        case ButtonPress:
            if (event.xbutton.button == Button1) { // Left click
                // Calculate which entry was clicked
                int x = event.xbutton.x;
                int y = event.xbutton.y;
                
                // Check for button clicks in header area first
                if (y < 140) {
                    int button_clicked = xlunch_native_check_button_hover(xl, x, y);
                    if (button_clicked == 1) {
                        return -2; // Settings button clicked
                    } else if (button_clicked == 2) {
                        return -3; // Logo button clicked
                    }
                    break; // Other header clicks ignored
                }
                
                // Adjust coordinates for the grid area - match exact drawing logic
                int grid_y = y - 140; // Skip header
                
                if (x >= 0 && grid_y >= 0) {
                    int col = x / xl->cell_width;
                    int display_row = grid_y / xl->cell_height; // This is the visible row (0-based)
                    
                    // Calculate the actual entry index by accounting for scroll offset
                    // This matches exactly how the drawing logic works
                    int actual_row = display_row + xl->scrolled_past;
                    int index = actual_row * xl->cols + col;
                    
                    // Bounds check - ensure we're within valid ranges
                    if (col >= 0 && col < xl->cols && display_row >= 0 && 
                        index >= 0 && index < xl->num_entries) {
                        
                        // Additional check: make sure this entry would actually be drawn
                        // (matching the drawing logic's visibility checks)
                        int startY = 140;
                        int cellHeight = xl->cell_height;
                        int maxVisibleRows = (xl->height - startY) / cellHeight;
                        
                        if (display_row < maxVisibleRows) {
                            *selected_entry = index;
                            return 0; // Exit with selection
                        }
                    }
                }
            } else if (event.xbutton.button == Button4) { // Mouse wheel up (like xlunch)
                int scroll_changed = xlunch_native_set_scroll_level(xl, xl->scrolled_past - 1);
                return scroll_changed ? 2 : 1; // Only redraw if scroll actually changed
            } else if (event.xbutton.button == Button5) { // Mouse wheel down (like xlunch)
                int scroll_changed = xlunch_native_set_scroll_level(xl, xl->scrolled_past + 1);
                return scroll_changed ? 2 : 1; // Only redraw if scroll actually changed
            }
            break;

        case Expose:
            return 2; // Redraw needed

        case ClientMessage:
            // Handle window close button
            return 0; // Quit

        case MotionNotify:
            // Handle mouse movement for hover effects (but don't trigger full redraws)
            xlunch_native_handle_hover(xl, event.xmotion.x, event.xmotion.y);
            // Don't return 2 (redraw) for every mouse movement - only for significant changes
            break;

        case EnterNotify:
        case LeaveNotify:
            // Mouse enter/leave window - reset hover states
            xl->settings_button_hovered = 0;
            xl->logo_button_hovered = 0;
            break;

        default:
            // Unknown event - ignore
            break;
        }
    }
    return 1; // Continue
}

// Check if mouse is over a button (returns 1 for settings, 2 for logo, 0 for none)
int xlunch_native_check_button_hover(XLunchNative *xl, int mouse_x, int mouse_y) {
    if (!xl) return 0;
    
    // Check settings button
    if (mouse_x >= xl->settings_x && mouse_x <= xl->settings_x + xl->settings_w &&
        mouse_y >= xl->settings_y && mouse_y <= xl->settings_y + xl->settings_h) {
        return 1;
    }
    
    // Check logo button (Pi-Apps logo text area)
    if (mouse_x >= xl->logo_x && mouse_x <= xl->logo_x + xl->logo_w &&
        mouse_y >= xl->logo_y && mouse_y <= xl->logo_y + xl->logo_h) {
        return 2;
    }
    
    return 0;
}

// Handle button hover effects
void xlunch_native_handle_hover(XLunchNative *xl, int mouse_x, int mouse_y) {
    if (!xl) return;
    
    int old_settings_hover = xl->settings_button_hovered;
    int old_logo_hover = xl->logo_button_hovered;
    
    // Reset hover states
    xl->settings_button_hovered = 0;
    xl->logo_button_hovered = 0;
    
    // Check which button is hovered
    int button = xlunch_native_check_button_hover(xl, mouse_x, mouse_y);
    if (button == 1) {
        xl->settings_button_hovered = 1;
    } else if (button == 2) {
        xl->logo_button_hovered = 1;
    }
    
    // Don't trigger automatic redraws for hover - let the main loop handle it
    // This prevents constant full window redraws on mouse movement
    (void)old_settings_hover; // Suppress unused variable warning
    (void)old_logo_hover;     // Suppress unused variable warning
}

// Draw button with hover effect
void xlunch_native_draw_button_with_hover(XLunchNative *xl, int x, int y, int width, int height, const char *icon_path, int hovered) {
    if (!xl || !icon_path) return;
    
    // Apply hover effect by making it slightly larger (like original xlunch)
    int hover_offset = hovered ? -2 : 0; // Make bigger when hovered
    int hover_size_increase = hovered ? 4 : 0;
    
    int draw_x = x + hover_offset;
    int draw_y = y + hover_offset;
    int draw_w = width + hover_size_increase;
    int draw_h = height + hover_size_increase;
    
    // No blue glow - just size change like original xlunch
    
    // Load and draw the icon
    Imlib_Image icon = imlib_load_image(icon_path);
    if (icon) {
        imlib_context_set_image(icon);
        int orig_w = imlib_image_get_width();
        int orig_h = imlib_image_get_height();
        
        // Calculate aspect ratio to prevent stretching
        float aspect_ratio = (float)orig_w / (float)orig_h;
        int final_w, final_h;
        
        if (aspect_ratio > 1.0f) {
            // Image is wider than tall
            final_w = draw_w;
            final_h = (int)(draw_w / aspect_ratio);
            if (final_h > draw_h) {
                final_h = draw_h;
                final_w = (int)(draw_h * aspect_ratio);
            }
        } else {
            // Image is taller than wide or square
            final_h = draw_h;
            final_w = (int)(draw_h * aspect_ratio);
            if (final_w > draw_w) {
                final_w = draw_w;
                final_h = (int)(draw_w / aspect_ratio);
            }
        }
        
        // Center the image in the available space
        int center_x = draw_x + (draw_w - final_w) / 2;
        int center_y = draw_y + (draw_h - final_h) / 2;
        
        imlib_context_set_drawable(xl->window);
        imlib_context_set_blend(1);
        imlib_render_image_on_drawable_at_size(center_x, center_y, final_w, final_h);
        imlib_free_image();
    }
}

// Draw rounded rectangle (simulate rounded corners)
void xlunch_native_draw_rounded_rect(XLunchNative *xl, int x, int y, int width, int height, int radius, unsigned long color) {
    if (!xl) return;

    XSetForeground(xl->display, xl->gc, color);

    // Draw main rectangle body
    XFillRectangle(xl->display, xl->window, xl->gc, x + radius, y, width - 2*radius, height);
    XFillRectangle(xl->display, xl->window, xl->gc, x, y + radius, width, height - 2*radius);

    // Draw corner arcs (simplified - just cut off sharp corners)
    for (int i = 0; i < radius; i++) {
        for (int j = 0; j < radius; j++) {
            double dist = sqrt(i*i + j*j);
            if (dist <= radius) {
                // Top-left corner
                XDrawPoint(xl->display, xl->window, xl->gc, x + radius - i, y + radius - j);
                // Top-right corner
                XDrawPoint(xl->display, xl->window, xl->gc, x + width - radius + i - 1, y + radius - j);
                // Bottom-left corner
                XDrawPoint(xl->display, xl->window, xl->gc, x + radius - i, y + height - radius + j - 1);
                // Bottom-right corner
                XDrawPoint(xl->display, xl->window, xl->gc, x + width - radius + i - 1, y + height - radius + j - 1);
            }
        }
    }
}

// Set scroll level (like xlunch set_scroll_level) - returns 1 if changed, 0 if no change
int xlunch_native_set_scroll_level(XLunchNative *xl, int new_scroll) {
    if (!xl || xl->noscroll) return 0;
    
    int old_scroll = xl->scrolled_past;
    
    if (new_scroll != xl->scrolled_past) {
        xl->scrolled_past = new_scroll;
        
        // Calculate maximum scroll (like xlunch)
        int max_scroll = (xl->entries_count - 1) / xl->cols - xl->rows + 1;
        if (xl->scrolled_past > max_scroll) {
            xl->scrolled_past = max_scroll;
        }
        if (xl->scrolled_past < 0) {
            xl->scrolled_past = 0;
        }
        
        // Return 1 if scroll position actually changed
        return (xl->scrolled_past != old_scroll) ? 1 : 0;
    }
    
    return 0; // No change
}

// Draw scrollbar (like xlunch scrollbar drawing)
void xlunch_native_draw_scrollbar(XLunchNative *xl) {
    if (!xl || xl->noscroll) return;
    
    // Calculate scrollbar dimensions (like xlunch)
    int scrollbar_width = 15;
    int scrollbar_height = xl->height - 180; // Account for header and margins
    int scrollbar_screen_margin = 30; // Closer to edge like xlunch
    
    // Calculate total pages needed
    int visible_entries = xl->rows * xl->cols;
    if (xl->entries_count <= visible_entries) return; // No scrollbar needed
    
    // Calculate scrollbar indicator size and position (like xlunch)
    int scrollbar_draggable_height = (scrollbar_height * visible_entries) / xl->entries_count;
    if (scrollbar_draggable_height < 20) scrollbar_draggable_height = 20; // Minimum size
    
    // Calculate current position
    float scroll_percentage = (float)xl->scrolled_past / ((xl->entries_count - 1) / xl->cols - xl->rows + 1);
    int scrollbar_draggable_shift = (int)((scrollbar_height - scrollbar_draggable_height) * scroll_percentage);
    
    // Use proper semi-transparent colors (like xlunch: RGB with alpha)
    // Draw scrollbar background (semi-transparent white)
    XSetForeground(xl->display, xl->gc, 0x606060); // Dark gray background
    XFillRectangle(xl->display, xl->window, xl->gc,
                   xl->width - scrollbar_screen_margin, 160, // Start below header
                   scrollbar_width, scrollbar_height);
    
    // Draw current scroll position indicator (lighter gray)
    XSetForeground(xl->display, xl->gc, 0xA0A0A0); // Light gray indicator
    XFillRectangle(xl->display, xl->window, xl->gc,
                   xl->width - scrollbar_screen_margin, 160 + scrollbar_draggable_shift,
                   scrollbar_width, scrollbar_draggable_height);
}

// Calculate entry position with scroll offset (like xlunch arrange_positions)
void xlunch_native_calculate_entry_position(XLunchNative *xl, int entry_index, int *x, int *y) {
    if (!xl || !x || !y) return;
    
    // Calculate row and column (0-based)
    int row = entry_index / xl->cols;
    int col = entry_index % xl->cols;
    
    // Apply scroll offset (like xlunch)
    int display_row = row - xl->scrolled_past;
    
    // Calculate position
    *x = col * xl->cell_width + (xl->cell_width - xl->icon_size) / 2;
    *y = 140 + display_row * xl->cell_height; // 140 = header height
}

// Cleanup
void xlunch_native_cleanup(XLunchNative *xl) {
    if (!xl) return;

    if (xl->background_image) {
        imlib_context_set_image(xl->background_image);
        imlib_free_image();
    }
    
    // Clean up render buffer for double buffering
    if (xl->render_buffer) {
        imlib_context_set_image(xl->render_buffer);
        imlib_free_image();
    }
    
    if (xl->font) XftFontClose(xl->display, xl->font);
    if (xl->xft_draw) XftDrawDestroy(xl->xft_draw);
    if (xl->gc) XFreeGC(xl->display, xl->gc);
    if (xl->window) XDestroyWindow(xl->display, xl->window);
    if (xl->display) XCloseDisplay(xl->display);

    XftColorFree(xl->display, xl->vinfo.visual, xl->colormap, &xl->text_color);
    XftColorFree(xl->display, xl->vinfo.visual, xl->colormap, &xl->highlight_color);
    XftColorFree(xl->display, xl->vinfo.visual, xl->colormap, &xl->background_color);

    free(xl);
} 