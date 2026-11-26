import * as ImagePicker from 'expo-image-picker';
import { File } from 'expo-file-system';
import { MAX_PROFILE_PHOTO_BYTES, photoContentType, ProfileError } from './profileApi';
import type { ProfilePhoto } from './profileApi';

// Library selection only: no camera/microphone access, EXIF or base64 request.
export async function pickProfilePhoto(): Promise<ProfilePhoto | null> {
  const result = await ImagePicker.launchImageLibraryAsync({ mediaTypes: ['images'], allowsMultipleSelection: false, allowsEditing: false, base64: false, exif: false, quality: 1 });
  if (result.canceled) return null;
  const asset = result.assets[0];
  if (!asset || !asset.uri.startsWith('file://')) throw new ProfileError(415, 'This photo could not be read. Choose a JPEG or PNG saved on your phone.');
  const file = new File(asset.uri);
  if ((asset.fileSize ?? 0) > MAX_PROFILE_PHOTO_BYTES || file.size > MAX_PROFILE_PHOTO_BYTES) throw new ProfileError(413, 'Choose a photo up to 2 MiB.');
  const bytes = await file.bytes();
  const contentType = photoContentType(bytes);
  return { uri: asset.uri, data: new Uint8Array(bytes).buffer, contentType };
}
