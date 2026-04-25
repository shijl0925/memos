import classNames from "classnames";

interface Props {
  avatarUrl?: string;
  className?: string;
}

const UserAvatar = (props: Props) => {
  const { avatarUrl, className } = props;
  return (
    <div className={classNames("flex items-center justify-center overflow-hidden rounded-full", className ?? "w-8 h-8")}>
      <img className="w-full h-full object-cover rounded-full" src={avatarUrl || "/logo.webp"} alt="" />
    </div>
  );
};

export default UserAvatar;
