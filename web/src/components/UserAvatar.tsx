import classNames from "classnames";
import { DEFAULT_MEMOS_LOGO_URL } from "@/helpers/consts";

interface Props {
  avatarUrl?: string;
  className?: string;
}

const UserAvatar = (props: Props) => {
  const { avatarUrl, className } = props;
  return (
    <div className={classNames("flex items-center justify-center overflow-hidden rounded-full", className ?? "w-8 h-8")}>
      <img className="w-full h-full object-cover rounded-full" src={avatarUrl || DEFAULT_MEMOS_LOGO_URL} alt="" />
    </div>
  );
};

export default UserAvatar;
