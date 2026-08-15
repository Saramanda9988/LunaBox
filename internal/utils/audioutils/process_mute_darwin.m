//go:build darwin && cgo

#import <CoreAudio/CoreAudio.h>
#import <CoreAudio/AudioHardwareTapping.h>
#import <CoreAudio/CATapDescription.h>
#import <Foundation/Foundation.h>
#include <limits.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

enum {
    LunaBoxProcessMuteSuccess = 0,
    LunaBoxProcessMuteUnavailable = 1,
    LunaBoxProcessMuteProcessNotFound = 2,
    LunaBoxProcessMuteFailure = 3,
};

typedef struct {
    AudioObjectID tap_id;
    AudioObjectID aggregate_device_id;
    AudioDeviceIOProcID io_proc_id;
} LunaBoxProcessMuteTap;

static OSStatus lunabox_process_mute_io_proc(
    AudioObjectID device_id,
    const AudioTimeStamp *current_time,
    const AudioBufferList *input_data,
    const AudioTimeStamp *input_time,
    AudioBufferList *output_data,
    const AudioTimeStamp *output_time,
    void *client_data
) {
    (void)device_id;
    (void)current_time;
    (void)input_data;
    (void)input_time;
    (void)output_time;
    (void)client_data;
    if (output_data != NULL) {
        for (UInt32 index = 0; index < output_data->mNumberBuffers; index++) {
            AudioBuffer *buffer = &output_data->mBuffers[index];
            if (buffer->mData != NULL && buffer->mDataByteSize > 0) {
                memset(buffer->mData, 0, buffer->mDataByteSize);
            }
        }
    }
    return noErr;
}

static bool lunabox_cleanup_status_is_ignorable(OSStatus status) {
    return status == noErr ||
        status == kAudioHardwareBadObjectError ||
        status == kAudioHardwareNotRunningError;
}

static void lunabox_record_cleanup_status(OSStatus status, OSStatus *first_error) {
    if (!lunabox_cleanup_status_is_ignorable(status) && *first_error == noErr) {
        *first_error = status;
    }
}

int32_t lunabox_process_mute_supported(void) {
    if (@available(macOS 14.2, *)) {
        return 1;
    }
    return 0;
}

int32_t lunabox_create_process_mute_tap(uint32_t process_id, uintptr_t *tap_handle, int32_t *os_status) {
    if (tap_handle == NULL || os_status == NULL || process_id == 0 || process_id > INT32_MAX) {
        return LunaBoxProcessMuteFailure;
    }

    *tap_handle = 0;
    *os_status = noErr;

    if (@available(macOS 14.2, *)) {
        @autoreleasepool {
            pid_t target_pid = (pid_t)process_id;
            AudioObjectID process_object_id = kAudioObjectUnknown;
            UInt32 property_size = sizeof(process_object_id);
            AudioObjectPropertyAddress address = {
                kAudioHardwarePropertyTranslatePIDToProcessObject,
                kAudioObjectPropertyScopeGlobal,
                kAudioObjectPropertyElementMain,
            };

            OSStatus status = AudioObjectGetPropertyData(
                kAudioObjectSystemObject,
                &address,
                sizeof(target_pid),
                &target_pid,
                &property_size,
                &process_object_id
            );
            if (status != noErr || process_object_id == kAudioObjectUnknown) {
                *os_status = status;
                return LunaBoxProcessMuteProcessNotFound;
            }

            CATapDescription *description = [[CATapDescription alloc] initStereoMixdownOfProcesses:@[@(process_object_id)]];
            [description setUUID:[NSUUID UUID]];
            [description setName:[NSString stringWithFormat:@"LunaBox background mute %u", process_id]];
            [description setPrivate:YES];
            [description setMuteBehavior:CATapMutedWhenTapped];

            AudioObjectID created_tap_id = kAudioObjectUnknown;
            status = AudioHardwareCreateProcessTap(description, &created_tap_id);
            if (status != noErr || created_tap_id == kAudioObjectUnknown) {
                [description release];
                *os_status = status;
                return LunaBoxProcessMuteFailure;
            }

            AudioObjectID output_device_id = kAudioObjectUnknown;
            property_size = sizeof(output_device_id);
            address = (AudioObjectPropertyAddress){
                kAudioHardwarePropertyDefaultSystemOutputDevice,
                kAudioObjectPropertyScopeGlobal,
                kAudioObjectPropertyElementMain,
            };
            status = AudioObjectGetPropertyData(
                kAudioObjectSystemObject,
                &address,
                0,
                NULL,
                &property_size,
                &output_device_id
            );
            if (status != noErr || output_device_id == kAudioObjectUnknown) {
                AudioHardwareDestroyProcessTap(created_tap_id);
                [description release];
                *os_status = status;
                return LunaBoxProcessMuteFailure;
            }

            CFStringRef output_device_uid = NULL;
            property_size = sizeof(output_device_uid);
            address = (AudioObjectPropertyAddress){
                kAudioDevicePropertyDeviceUID,
                kAudioObjectPropertyScopeGlobal,
                kAudioObjectPropertyElementMain,
            };
            status = AudioObjectGetPropertyData(
                output_device_id,
                &address,
                0,
                NULL,
                &property_size,
                &output_device_uid
            );
            if (status != noErr || output_device_uid == NULL) {
                AudioHardwareDestroyProcessTap(created_tap_id);
                [description release];
                *os_status = status;
                return LunaBoxProcessMuteFailure;
            }

            NSString *tap_uid = [[description UUID] UUIDString];
            NSDictionary *aggregate_description = @{
                @kAudioAggregateDeviceNameKey: [NSString stringWithFormat:@"LunaBox mute %u", process_id],
                @kAudioAggregateDeviceUIDKey: [[NSUUID UUID] UUIDString],
                @kAudioAggregateDeviceMainSubDeviceKey: (NSString *)output_device_uid,
                @kAudioAggregateDeviceIsPrivateKey: @(YES),
                @kAudioAggregateDeviceIsStackedKey: @(NO),
                @kAudioAggregateDeviceTapAutoStartKey: @(YES),
                @kAudioAggregateDeviceSubDeviceListKey: @[
                    @{
                        @kAudioSubDeviceUIDKey: (NSString *)output_device_uid,
                    },
                ],
                @kAudioAggregateDeviceTapListKey: @[
                    @{
                        @kAudioSubTapDriftCompensationKey: @(YES),
                        @kAudioSubTapUIDKey: tap_uid,
                    },
                ],
            };

            AudioObjectID aggregate_device_id = kAudioObjectUnknown;
            status = AudioHardwareCreateAggregateDevice(
                (CFDictionaryRef)aggregate_description,
                &aggregate_device_id
            );
            CFRelease(output_device_uid);
            [description release];
            if (status != noErr || aggregate_device_id == kAudioObjectUnknown) {
                AudioHardwareDestroyProcessTap(created_tap_id);
                *os_status = status;
                return LunaBoxProcessMuteFailure;
            }

            AudioDeviceIOProcID io_proc_id = NULL;
            status = AudioDeviceCreateIOProcID(
                aggregate_device_id,
                lunabox_process_mute_io_proc,
                NULL,
                &io_proc_id
            );
            if (status != noErr || io_proc_id == NULL) {
                AudioHardwareDestroyAggregateDevice(aggregate_device_id);
                AudioHardwareDestroyProcessTap(created_tap_id);
                *os_status = status;
                return LunaBoxProcessMuteFailure;
            }

            status = AudioDeviceStart(aggregate_device_id, io_proc_id);
            if (status != noErr) {
                AudioDeviceDestroyIOProcID(aggregate_device_id, io_proc_id);
                AudioHardwareDestroyAggregateDevice(aggregate_device_id);
                AudioHardwareDestroyProcessTap(created_tap_id);
                *os_status = status;
                return LunaBoxProcessMuteFailure;
            }

            LunaBoxProcessMuteTap *tap = calloc(1, sizeof(LunaBoxProcessMuteTap));
            if (tap == NULL) {
                AudioDeviceStop(aggregate_device_id, io_proc_id);
                AudioDeviceDestroyIOProcID(aggregate_device_id, io_proc_id);
                AudioHardwareDestroyAggregateDevice(aggregate_device_id);
                AudioHardwareDestroyProcessTap(created_tap_id);
                *os_status = kAudioHardwareUnspecifiedError;
                return LunaBoxProcessMuteFailure;
            }

            tap->tap_id = created_tap_id;
            tap->aggregate_device_id = aggregate_device_id;
            tap->io_proc_id = io_proc_id;
            *tap_handle = (uintptr_t)tap;
            return LunaBoxProcessMuteSuccess;
        }
    }

    return LunaBoxProcessMuteUnavailable;
}

int32_t lunabox_destroy_process_mute_tap(uintptr_t tap_handle, int32_t *os_status) {
    if (os_status == NULL || tap_handle == 0) {
        return LunaBoxProcessMuteFailure;
    }

    *os_status = noErr;
    if (@available(macOS 14.2, *)) {
        LunaBoxProcessMuteTap *tap = (LunaBoxProcessMuteTap *)tap_handle;
        OSStatus first_error = noErr;
        lunabox_record_cleanup_status(
            AudioDeviceStop(tap->aggregate_device_id, tap->io_proc_id),
            &first_error
        );
        lunabox_record_cleanup_status(
            AudioDeviceDestroyIOProcID(tap->aggregate_device_id, tap->io_proc_id),
            &first_error
        );
        lunabox_record_cleanup_status(
            AudioHardwareDestroyAggregateDevice(tap->aggregate_device_id),
            &first_error
        );
        lunabox_record_cleanup_status(
            AudioHardwareDestroyProcessTap(tap->tap_id),
            &first_error
        );
        free(tap);
        if (first_error != noErr) {
            *os_status = first_error;
            return LunaBoxProcessMuteFailure;
        }
        return LunaBoxProcessMuteSuccess;
    }

    return LunaBoxProcessMuteUnavailable;
}
